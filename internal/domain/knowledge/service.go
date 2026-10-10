package knowledge

import (
	"context"
	"math"
	"strings"
	"time"
	"unicode"

	"github.com/mk20mm/homeagent/internal/apperr"
)

// Visibility 可见范围
const (
	VisibilityFamily  = "family"  // 家庭共享
	VisibilityPrivate = "private" // 个人私密
)

// Document 知识库文档实体
type Document struct {
	ID            string
	Title         string
	Category      string
	Visibility    string
	Content       string
	FilePath      string
	FileSizeBytes int64
	ChunkCount    int
	MemberID      string
	AuthorName    string
	Status        string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// Chunk 知识库切片实体
type Chunk struct {
	ID         string
	DocumentID string
	ChunkIndex int
	Content    string
	TokenCount int
	Embedding  []byte
	CreatedAt  time.Time
}

// ChunkWithDoc 携带文档元数据的切片（用于混合检索召回）
type ChunkWithDoc struct {
	Chunk
	DocTitle      string
	DocCategory   string
	DocVisibility string
	DocMemberID   string
}

// QueryResult 检索结果
type QueryResult struct {
	DocumentID    string
	DocumentTitle string
	Category      string
	Visibility    string
	ChunkIndex    int
	Content       string
	Score         float64
	IsPrivate     bool
}

// Space 知识空间概览
type Space struct {
	ID            string
	Name          string
	Visibility    string
	DocumentCount int
	Description   string
	IsPersonal    bool
}

// CreateDocumentCmd 创建文档命令
type CreateDocumentCmd struct {
	Title      string
	Category   string
	Visibility string
	Content    string
}

// Repo 知识库数据仓储接口
type Repo interface {
	CreateDocument(ctx context.Context, doc Document, chunks []Chunk) (Document, error)
	GetDocument(ctx context.Context, id string) (Document, error)
	ListDocuments(ctx context.Context, memberID string, visibility string, category string, keyword string) ([]Document, error)
	DeleteDocument(ctx context.Context, id string) error
	RestoreDocument(ctx context.Context, id string) error
	QueryChunks(ctx context.Context, memberID string, category string) ([]ChunkWithDoc, error)
	CountDocumentsBySpace(ctx context.Context, memberID string) (familyCount int, privateCount int, err error)
}

// Service 知识库领域服务接口
type Service interface {
	ListSpaces(ctx context.Context, memberID string) ([]Space, error)
	ListDocuments(ctx context.Context, memberID string, visibility string, category string, keyword string) ([]Document, error)
	CreateDocument(ctx context.Context, memberID string, authorName string, cmd CreateDocumentCmd) (Document, error)
	GetDocument(ctx context.Context, memberID string, id string) (Document, error)
	DeleteDocument(ctx context.Context, memberID string, id string) (undoID string, err error)
	RestoreDocument(ctx context.Context, id string) error
	Query(ctx context.Context, memberID string, query string, category string, topK int) ([]QueryResult, error)
}

type service struct {
	repo Repo
}

func NewService(repo Repo) Service {
	return &service{repo: repo}
}

// ListSpaces 获取当前成员有权访问的空间（家庭共享 + 个人私密）
func (s *service) ListSpaces(ctx context.Context, memberID string) ([]Space, error) {
	familyCount, privateCount, err := s.repo.CountDocumentsBySpace(ctx, memberID)
	if err != nil {
		return nil, err
	}

	spaces := []Space{
		{
			ID:            "family",
			Name:          "家庭共享空间",
			Visibility:    VisibilityFamily,
			DocumentCount: familyCount,
			Description:   "全家成员可见与可检索，包含家电说明书、家庭保单、合同与公共生活备忘",
			IsPersonal:    false,
		},
		{
			ID:            "private",
			Name:          "个人私密空间",
			Visibility:    VisibilityPrivate,
			DocumentCount: privateCount,
			Description:   "专属私密空间，仅本人与授权会话可检索，保护体检档案与个人私密备忘",
			IsPersonal:    true,
		},
	}
	return spaces, nil
}

// ListDocuments 浏览文档列表（执行严格权限过滤）
func (s *service) ListDocuments(ctx context.Context, memberID string, visibility string, category string, keyword string) ([]Document, error) {
	return s.repo.ListDocuments(ctx, memberID, visibility, category, keyword)
}

// CreateDocument 创建文档并切片建索引
func (s *service) CreateDocument(ctx context.Context, memberID string, authorName string, cmd CreateDocumentCmd) (Document, error) {
	title := strings.TrimSpace(cmd.Title)
	if title == "" {
		return Document{}, apperr.New(apperr.CodeInvalidInput, "文档标题不能为空", nil)
	}
	content := strings.TrimSpace(cmd.Content)
	if content == "" {
		return Document{}, apperr.New(apperr.CodeInvalidInput, "文档正文内容不能为空", nil)
	}

	visibility := cmd.Visibility
	if visibility != VisibilityPrivate {
		visibility = VisibilityFamily
	}

	category := strings.TrimSpace(cmd.Category)
	if category == "" {
		category = "manual"
	}

	// 智能切片（400 字滑动窗口 + 50 字重叠）
	rawChunks := ChunkText(content, 400, 50)
	chunks := make([]Chunk, len(rawChunks))
	for i, c := range rawChunks {
		chunks[i] = Chunk{
			ChunkIndex: i,
			Content:    c,
			TokenCount: len([]rune(c)) / 2, // 中英文混合近似估算
		}
	}

	doc := Document{
		Title:         title,
		Category:      category,
		Visibility:    visibility,
		Content:       content,
		FileSizeBytes: int64(len(content)),
		ChunkCount:    len(chunks),
		MemberID:      memberID,
		AuthorName:    authorName,
		Status:        "indexed",
	}

	return s.repo.CreateDocument(ctx, doc, chunks)
}

// GetDocument 获取文档详情（执行权限校验）
func (s *service) GetDocument(ctx context.Context, memberID string, id string) (Document, error) {
	doc, err := s.repo.GetDocument(ctx, id)
	if err != nil {
		return Document{}, err
	}

	// 权限双保险：私密文档若非上传者本人，拒绝访问
	if doc.Visibility == VisibilityPrivate && doc.MemberID != memberID {
		return Document{}, apperr.New(apperr.CodePermission, "无权访问此私密文档", nil)
	}

	return doc, nil
}

// DeleteDocument 删除文档（可撤销）
func (s *service) DeleteDocument(ctx context.Context, memberID string, id string) (string, error) {
	doc, err := s.repo.GetDocument(ctx, id)
	if err != nil {
		return "", err
	}

	// 仅本人或公共文档可操作
	if doc.Visibility == VisibilityPrivate && doc.MemberID != memberID {
		return "", apperr.New(apperr.CodePermission, "无权删除他人的私密文档", nil)
	}

	if err := s.repo.DeleteDocument(ctx, id); err != nil {
		return "", err
	}

	return id, nil
}

// RestoreDocument 撤销恢复已删除文档
func (s *service) RestoreDocument(ctx context.Context, id string) error {
	return s.repo.RestoreDocument(ctx, id)
}

// Query 混合检索（Hybrid RAG：关键词精准度 + 语义重合度 + 权限硬过滤）
func (s *service) Query(ctx context.Context, memberID string, query string, category string, topK int) ([]QueryResult, error) {
	q := strings.TrimSpace(query)
	if q == "" {
		return []QueryResult{}, nil
	}
	if topK <= 0 {
		topK = 3
	}
	if topK > 5 {
		topK = 5
	}

	// 从仓储中拉取所有候选切片（SQL 层面已通过 memberID 与 visibility 过滤）
	candidates, err := s.repo.QueryChunks(ctx, memberID, category)
	if err != nil {
		return nil, err
	}
	if len(candidates) == 0 {
		return []QueryResult{}, nil
	}

	queryTokens := tokenize(q)
	type scoredCandidate struct {
		chunk ChunkWithDoc
		score float64
	}

	var scored []scoredCandidate
	for _, c := range candidates {
		score := calculateRelevance(q, queryTokens, c)
		if score > 0.05 { // 过滤完全无关的切片
			scored = append(scored, scoredCandidate{chunk: c, score: score})
		}
	}

	// 冒泡/选择排序 Top K
	for i := 0; i < len(scored); i++ {
		for j := i + 1; j < len(scored); j++ {
			if scored[j].score > scored[i].score {
				scored[i], scored[j] = scored[j], scored[i]
			}
		}
	}

	limit := topK
	if limit > len(scored) {
		limit = len(scored)
	}

	results := make([]QueryResult, limit)
	for i := 0; i < limit; i++ {
		item := scored[i]
		results[i] = QueryResult{
			DocumentID:    item.chunk.DocumentID,
			DocumentTitle: item.chunk.DocTitle,
			Category:      item.chunk.DocCategory,
			Visibility:    item.chunk.DocVisibility,
			ChunkIndex:    item.chunk.ChunkIndex,
			Content:       item.chunk.Content,
			Score:         math.Round(item.score*100) / 100,
			IsPrivate:     item.chunk.DocVisibility == VisibilityPrivate,
		}
	}

	return results, nil
}

// ChunkText 纯 Go 智能文本切片（滑窗策略，保持句子或段落完整度）
func ChunkText(text string, maxChunkLen int, overlap int) []string {
	runes := []rune(text)
	total := len(runes)
	if total <= maxChunkLen {
		return []string{text}
	}

	var chunks []string
	step := maxChunkLen - overlap
	if step <= 0 {
		step = maxChunkLen / 2
	}

	for start := 0; start < total; start += step {
		end := start + maxChunkLen
		if end > total {
			end = total
		}

		// 尽量在标点符号处断句
		if end < total {
			adjusted := end
			for i := end; i > start+step/2; i-- {
				r := runes[i-1]
				if r == '\n' || r == '。' || r == '；' || r == '！' || r == '？' || r == '.' || r == ';' {
					adjusted = i
					break
				}
			}
			end = adjusted
		}

		chunkStr := strings.TrimSpace(string(runes[start:end]))
		if len(chunkStr) > 0 {
			chunks = append(chunks, chunkStr)
		}

		if end >= total {
			break
		}
	}

	return chunks
}

// 分词/提取关键词（支持中文字符与英文单词）
func tokenize(s string) []string {
	var tokens []string
	var current strings.Builder

	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			if r < 128 {
				current.WriteRune(unicode.ToLower(r))
			} else {
				if current.Len() > 0 {
					tokens = append(tokens, current.String())
					current.Reset()
				}
				tokens = append(tokens, string(r)) // 单中文汉字
			}
		} else {
			if current.Len() > 0 {
				tokens = append(tokens, current.String())
				current.Reset()
			}
		}
	}
	if current.Len() > 0 {
		tokens = append(tokens, current.String())
	}
	return tokens
}

// calculateRelevance 混合打分算法（包含标题匹配、型号与专有名词精准命中、词频覆盖度）
func calculateRelevance(rawQuery string, queryTokens []string, c ChunkWithDoc) float64 {
	contentLower := strings.ToLower(c.Content)
	titleLower := strings.ToLower(c.DocTitle)
	score := 0.0

	// 1. 标题完全包含或关键词命中权重 (35%)
	if strings.Contains(titleLower, strings.ToLower(rawQuery)) {
		score += 0.35
	} else {
		titleHit := 0
		for _, t := range queryTokens {
			if len(t) > 1 && strings.Contains(titleLower, t) {
				titleHit++
			}
		}
		if len(queryTokens) > 0 {
			score += 0.20 * (float64(titleHit) / float64(len(queryTokens)))
		}
	}

	// 2. 正文精准子串匹配 (30%)
	if strings.Contains(contentLower, strings.ToLower(rawQuery)) {
		score += 0.30
	}

	// 3. 关键词覆盖率 (35%)
	matchedCount := 0
	for _, token := range queryTokens {
		if strings.Contains(contentLower, token) {
			matchedCount++
		}
	}
	if len(queryTokens) > 0 {
		score += 0.35 * (float64(matchedCount) / float64(len(queryTokens)))
	}

	// 4. 数字/英文型号（如 RO-800G、Wi-Fi 等）加权奖励
	for _, token := range queryTokens {
		if len(token) >= 3 && containsDigit(token) && strings.Contains(contentLower, token) {
			score += 0.15
			break
		}
	}

	if score > 1.0 {
		score = 1.0
	}
	return score
}

func containsDigit(s string) bool {
	for _, r := range s {
		if unicode.IsDigit(r) {
			return true
		}
	}
	return false
}
