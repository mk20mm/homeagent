package gateway

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"sync"
	"time"

	dommodel "github.com/mk20mm/homeagent/internal/domain/model"
)

// ConnectionResolver 凭证解析器接口（由 domain/model 或 repo.ProviderStore 实现）。
type ConnectionResolver interface {
	ResolveConnection(ctx context.Context, modelIDOrName string) (conn dommodel.ProviderConnection, ok bool, err error)
}

// DynamicGateway 实现 Provider 接口，支持按请求动态解析连接与多供应商路由。
type DynamicGateway struct {
	resolver ConnectionResolver
	fallback Provider // 环境变量或脚本 provider

	mu      sync.RWMutex
	clients map[string]*OpenAIProvider // cache key: hash(provider + baseURL + apiKey)
}

// NewDynamicGateway 创建动态网关。
func NewDynamicGateway(resolver ConnectionResolver, fallback Provider) *DynamicGateway {
	return &DynamicGateway{
		resolver: resolver,
		fallback: fallback,
		clients:  make(map[string]*OpenAIProvider),
	}
}

func (g *DynamicGateway) Name() string {
	return "dynamic"
}

// Invalidate 清除所有缓存的客户端（当供应商配置更新时触发）。
func (g *DynamicGateway) Invalidate() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.clients = make(map[string]*OpenAIProvider)
	slog.Info("dynamic gateway: client cache invalidated")
}

// StreamChat 动态路由：根据 modelIDOrName 解析对应供应商连接并分发。
func (g *DynamicGateway) StreamChat(ctx context.Context, req ChatRequest) (<-chan StreamEvent, error) {
	if g.resolver != nil {
		conn, ok, err := g.resolver.ResolveConnection(ctx, req.Model)
		if err == nil && ok && conn.APIKey != "" {
			provider := g.getOrCreateClient(conn)
			// 将请求的模型名替换为真实的供应商底层模型名（如 deepseek-chat）
			req.Model = conn.Model
			return provider.StreamChat(ctx, req)
		}
	}

	// 无法解析出数据库配置时，走兜底 provider
	if g.fallback != nil {
		return g.fallback.StreamChat(ctx, req)
	}

	return nil, fmt.Errorf("no available llm provider configured")
}

// getOrCreateClient 取或建 OpenAIProvider 实例。
func (g *DynamicGateway) getOrCreateClient(conn dommodel.ProviderConnection) *OpenAIProvider {
	key := clientCacheKey(conn.Provider, conn.BaseURL, conn.APIKey)
	g.mu.RLock()
	client, exists := g.clients[key]
	g.mu.RUnlock()
	if exists {
		return client
	}

	g.mu.Lock()
	defer g.mu.Unlock()
	if client, exists = g.clients[key]; exists {
		return client
	}

	client = NewOpenAIProvider(conn.APIKey, conn.BaseURL, conn.Model, conn.Provider)
	g.clients[key] = client
	return client
}

func clientCacheKey(prov, baseURL, apiKey string) string {
	h := sha256.Sum256([]byte(prov + "|" + baseURL + "|" + apiKey))
	return hex.EncodeToString(h[:16])
}

// Probe 执行一次轻量探针测试，返回响应耗时（毫秒）。
func Probe(ctx context.Context, conn dommodel.ProviderConnection) (latencyMS int64, err error) {
	provider := NewOpenAIProvider(conn.APIKey, conn.BaseURL, conn.Model, conn.Provider)
	start := time.Now()
	timeoutCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	// 1. 优先尝试轻量级 ListModels 探针（标准 OpenAI 协议，不消耗 token 且不校验具体模型名）
	if models, listErr := provider.client.ListModels(timeoutCtx); listErr == nil && len(models.Models) > 0 {
		return time.Since(start).Milliseconds(), nil
	}

	// 2. 若中继不支持 ListModels，执行极简单轮 StreamChat
	ch, err := provider.StreamChat(timeoutCtx, ChatRequest{
		Model: conn.Model,
		Messages: []Message{
			{Role: RoleUser, Content: "hi"},
		},
		Temperature: 0.1,
	})
	if err != nil {
		return 0, err
	}

	// 等待第一个 chunk 或完成
	for e := range ch {
		if e.Err != nil {
			return 0, e.Err
		}
		if e.Delta != "" || e.Done {
			return time.Since(start).Milliseconds(), nil
		}
	}
	return time.Since(start).Milliseconds(), nil
}
