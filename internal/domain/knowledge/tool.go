package knowledge

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mk20mm/homeagent/internal/agent/tool"
	"github.com/mk20mm/homeagent/internal/apperr"
)

// QueryKnowledgeInput 检索入参
type QueryKnowledgeInput struct {
	Query    string `json:"query"`
	Category string `json:"category,omitempty"`
	TopK     int    `json:"top_k,omitempty"`
}

// QueryKnowledgeTool 检索工具（A1 只读工具）
type QueryKnowledgeTool struct {
	svc Service
}

func NewQueryKnowledgeTool(svc Service) *QueryKnowledgeTool {
	return &QueryKnowledgeTool{svc: svc}
}

func (t *QueryKnowledgeTool) Spec() tool.Spec {
	return tool.Spec{
		Name: "query_knowledge",
		Description: "检索家庭私域知识库（家电说明书、合同保单条款、家庭健康就医记录与生活资产备忘）。" +
			"当用户询问家中具体物品使用方法、净水器滤芯/报错、保单条款、Wi-Fi密码、资产位置等私域信息时调用。" +
			"何时不该用：用户询问天气、公开新闻、通用百科时请使用 web_search 而非本工具。",
		InputSchema: json.RawMessage(`{
  "type": "object",
  "required": ["query"],
  "properties": {
    "query": {
      "type": "string",
      "description": "检索关键词或问题句，如 '净水器滤芯型号'、'房屋租赁合同到期时间'"
    },
    "category": {
      "type": "string",
      "enum": ["all", "manual", "insurance", "contract", "health", "memo", "recipe"],
      "default": "all",
      "description": "可选限定分类：manual(说明书), insurance(保单), contract(合同), health(健康档案), memo(生活备忘), recipe(菜谱)"
    },
    "top_k": {
      "type": "integer",
      "minimum": 1,
      "maximum": 5,
      "default": 3,
      "description": "召回最多相关的文本片段数"
    }
  }
}`),
		Risk:       tool.RiskLow,
		Permission: "knowledge.read",
		Module:     "knowledge",
		Replayable: true,
	}
}

func (t *QueryKnowledgeTool) Execute(ctx context.Context, input json.RawMessage) (tool.Result, error) {
	in, err := tool.DecodeInput[QueryKnowledgeInput](input)
	if err != nil {
		return tool.Result{}, apperr.New(apperr.CodeInvalidInput, "参数解析失败", err)
	}

	memberID := tool.MemberIDFrom(ctx)
	results, err := t.svc.Query(ctx, memberID, in.Query, in.Category, in.TopK)
	if err != nil {
		return tool.Result{}, err
	}

	if len(results) == 0 {
		return tool.Result{
			Summary: fmt.Sprintf("在家庭知识库中未检索到与「%s」相关的资料", in.Query),
		}, nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("在家庭知识库中找到 %d 条相关参考资料：\n", len(results)))
	for i, r := range results {
		spaceTag := "家庭共享"
		if r.IsPrivate {
			spaceTag = "个人私密"
		}
		sb.WriteString(fmt.Sprintf("%d. [%s · %s] (相关度 %.0f%%): %s\n",
			i+1, spaceTag, r.DocumentTitle, r.Score*100, r.Content))
	}

	cardPayload, _ := json.Marshal(map[string]any{
		"type":    "knowledge_citation",
		"query":   in.Query,
		"results": results,
	})

	return tool.Result{
		Summary: sb.String(),
		Card:    cardPayload,
	}, nil
}

// SaveKnowledgeInput 知识沉淀入参
type SaveKnowledgeInput struct {
	Title      string `json:"title"`
	Content    string `json:"content"`
	Category   string `json:"category,omitempty"`
	Visibility string `json:"visibility,omitempty"`
}

// SaveKnowledgeTool 沉淀工具（写操作，实现 WriteTool + 可撤销）
type SaveKnowledgeTool struct {
	svc Service
}

func NewSaveKnowledgeTool(svc Service) *SaveKnowledgeTool {
	return &SaveKnowledgeTool{svc: svc}
}

func (t *SaveKnowledgeTool) Spec() tool.Spec {
	return tool.Spec{
		Name: "save_knowledge",
		Description: "将重要信息沉淀保存到家庭知识库。当用户明确要求『记入家庭备忘』、『保存为说明书』、" +
			"『存入我的私密健康档案』等知识沉淀指令时调用。",
		InputSchema: json.RawMessage(`{
  "type": "object",
  "required": ["title", "content"],
  "properties": {
    "title": {
      "type": "string",
      "description": "文档或备忘标题，如 '美的净水器滤芯型号与复位说明'"
    },
    "content": {
      "type": "string",
      "description": "正文内容（支持 Markdown 或多行纯文本）"
    },
    "category": {
      "type": "string",
      "enum": ["manual", "insurance", "contract", "health", "memo", "recipe"],
      "default": "manual",
      "description": "分类：manual(家电说明书), insurance(保单), contract(合同), health(健康), memo(备忘), recipe(私房菜谱)"
    },
    "visibility": {
      "type": "string",
      "enum": ["family", "private"],
      "default": "family",
      "description": "可见范围：family(家庭共享，全家可见), private(个人私密，仅本人与授权会话可见)"
    }
  }
}`),
		Risk:        tool.RiskMedium,
		Permission:  "knowledge.write",
		Module:      "knowledge",
		Idempotency: "user+title+category",
	}
}

func (t *SaveKnowledgeTool) Execute(ctx context.Context, input json.RawMessage) (tool.Result, error) {
	in, err := tool.DecodeInput[SaveKnowledgeInput](input)
	if err != nil {
		return tool.Result{}, apperr.New(apperr.CodeInvalidInput, "参数解析失败", err)
	}

	memberID := tool.MemberIDFrom(ctx)
	doc, err := t.svc.CreateDocument(ctx, memberID, "家庭成员", CreateDocumentCmd{
		Title:      in.Title,
		Category:   in.Category,
		Visibility: in.Visibility,
		Content:    in.Content,
	})
	if err != nil {
		return tool.Result{}, err
	}

	spaceName := "家庭共享空间"
	if doc.Visibility == VisibilityPrivate {
		spaceName = "个人私密空间"
	}

	summary := fmt.Sprintf("已将《%s》成功存入【%s】（分类: %s，已切片为 %d 个片段并完成索引）",
		doc.Title, spaceName, doc.Category, doc.ChunkCount)

	undoData, _ := json.Marshal(map[string]any{
		"document_id": doc.ID,
	})

	cardPayload, _ := json.Marshal(map[string]any{
		"type":        "knowledge_document_created",
		"document_id": doc.ID,
		"title":       doc.Title,
		"category":    doc.Category,
		"visibility":  doc.Visibility,
		"space_name":  spaceName,
		"chunk_count": doc.ChunkCount,
	})

	return tool.Result{
		Summary:  summary,
		Card:     cardPayload,
		UndoData: undoData,
	}, nil
}

// Undo 软删除文档（写操作 24h 可撤销）
func (t *SaveKnowledgeTool) Undo(ctx context.Context, undoData json.RawMessage) error {
	var d struct {
		DocumentID string `json:"document_id"`
	}
	if err := json.Unmarshal(undoData, &d); err != nil {
		return apperr.New(apperr.CodeInvalidInput, "撤销数据解析失败", err)
	}
	memberID := tool.MemberIDFrom(ctx)
	_, err := t.svc.DeleteDocument(ctx, memberID, d.DocumentID)
	return err
}
