package knowledge_test

import (
	"context"
	"testing"
	"time"

	"github.com/mk20mm/homeagent/internal/apperr"
	"github.com/mk20mm/homeagent/internal/domain/knowledge"
)

type mockRepo struct {
	docs   map[string]knowledge.Document
	chunks map[string][]knowledge.Chunk
}

func newMockRepo() *mockRepo {
	return &mockRepo{
		docs:   make(map[string]knowledge.Document),
		chunks: make(map[string][]knowledge.Chunk),
	}
}

func (m *mockRepo) CreateDocument(ctx context.Context, doc knowledge.Document, chunks []knowledge.Chunk) (knowledge.Document, error) {
	doc.ID = "doc-" + doc.Title
	doc.CreatedAt = time.Now()
	doc.UpdatedAt = time.Now()
	m.docs[doc.ID] = doc
	m.chunks[doc.ID] = chunks
	return doc, nil
}

func (m *mockRepo) GetDocument(ctx context.Context, id string) (knowledge.Document, error) {
	doc, ok := m.docs[id]
	if !ok {
		return knowledge.Document{}, apperr.New(apperr.CodeNotFound, "文档不存在", nil)
	}
	return doc, nil
}

func (m *mockRepo) ListDocuments(ctx context.Context, memberID string, visibility string, category string, keyword string) ([]knowledge.Document, error) {
	var list []knowledge.Document
	for _, d := range m.docs {
		if visibility == knowledge.VisibilityPrivate && (d.Visibility != knowledge.VisibilityPrivate || d.MemberID != memberID) {
			continue
		}
		if visibility == knowledge.VisibilityFamily && d.Visibility != knowledge.VisibilityFamily {
			continue
		}
		if visibility == "" || visibility == "all" {
			if d.Visibility == knowledge.VisibilityPrivate && d.MemberID != memberID {
				continue
			}
		}
		list = append(list, d)
	}
	return list, nil
}

func (m *mockRepo) DeleteDocument(ctx context.Context, id string) error {
	delete(m.docs, id)
	delete(m.chunks, id)
	return nil
}

func (m *mockRepo) RestoreDocument(ctx context.Context, id string) error {
	return nil
}

func (m *mockRepo) QueryChunks(ctx context.Context, memberID string, category string) ([]knowledge.ChunkWithDoc, error) {
	var results []knowledge.ChunkWithDoc
	for docID, chunks := range m.chunks {
		doc := m.docs[docID]
		if doc.Visibility == knowledge.VisibilityPrivate && doc.MemberID != memberID {
			continue
		}
		for _, c := range chunks {
			results = append(results, knowledge.ChunkWithDoc{
				Chunk:         c,
				DocTitle:      doc.Title,
				DocCategory:   doc.Category,
				DocVisibility: doc.Visibility,
				DocMemberID:   doc.MemberID,
			})
		}
	}
	return results, nil
}

func (m *mockRepo) CountDocumentsBySpace(ctx context.Context, memberID string) (int, int, error) {
	familyCount := 0
	privateCount := 0
	for _, d := range m.docs {
		if d.Visibility == knowledge.VisibilityFamily {
			familyCount++
		} else if d.Visibility == knowledge.VisibilityPrivate && d.MemberID == memberID {
			privateCount++
		}
	}
	return familyCount, privateCount, nil
}

func TestChunkText(t *testing.T) {
	longText := "第一段内容。这是关于美的净水器RO-800G的使用说明，当滤芯寿命耗尽时，指示灯会显示红色报警。请按住复位键3秒钟。第二段内容开始，这里描述了更换滤芯的具体操作规范。"
	chunks := knowledge.ChunkText(longText, 50, 10)
	if len(chunks) < 2 {
		t.Fatalf("expected multiple chunks, got %d", len(chunks))
	}
}

func TestKnowledge_PermissionIsolation(t *testing.T) {
	repo := newMockRepo()
	svc := knowledge.NewService(repo)
	ctx := context.Background()

	dadID := "member-dad"
	momID := "member-mom"

	// 1. 爸爸创建家庭共享文档
	_, err := svc.CreateDocument(ctx, dadID, "爸爸", knowledge.CreateDocumentCmd{
		Title:      "美的净水器使用说明书",
		Category:   "manual",
		Visibility: knowledge.VisibilityFamily,
		Content:    "净水器滤芯型号为 RO-800G，报错 E02 时长按复位键3秒可重置滤芯寿命。",
	})
	if err != nil {
		t.Fatalf("create family doc failed: %v", err)
	}

	// 2. 妈妈创建个人私密文档
	momDoc, err := svc.CreateDocument(ctx, momID, "妈妈", knowledge.CreateDocumentCmd{
		Title:      "妈妈个人体检报告备忘",
		Category:   "health",
		Visibility: knowledge.VisibilityPrivate,
		Content:    "甘油三酯指标2.1偏高，少吃油腻食物，遵医嘱复查。",
	})
	if err != nil {
		t.Fatalf("create private doc failed: %v", err)
	}

	// 3. 爸爸列出文档：只能看到共享文档，绝不能看到妈妈的私密文档！
	dadDocs, err := svc.ListDocuments(ctx, dadID, "all", "", "")
	if err != nil {
		t.Fatalf("dad list docs failed: %v", err)
	}
	if len(dadDocs) != 1 || dadDocs[0].Title != "美的净水器使用说明书" {
		t.Fatalf("dad should only see family doc, got: %+v", dadDocs)
	}

	// 4. 妈妈列出文档：能看到共享文档和自己的私密文档
	momDocs, err := svc.ListDocuments(ctx, momID, "all", "", "")
	if err != nil {
		t.Fatalf("mom list docs failed: %v", err)
	}
	if len(momDocs) != 2 {
		t.Fatalf("mom should see 2 docs, got %d", len(momDocs))
	}

	// 5. 爸爸尝试直接按 ID 获取妈妈的私密文档 -> 必须被 403 Forbidden 拦截！
	_, err = svc.GetDocument(ctx, dadID, momDoc.ID)
	if err == nil {
		t.Fatalf("dad accessing mom's private doc should fail with forbidden")
	}

	// 6. 检索隔离测试：爸爸检索“甘油三酯”，检索结果必须为空（不能越权泄露）
	queryDad, err := svc.Query(ctx, dadID, "甘油三酯", "all", 3)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}
	if len(queryDad) != 0 {
		t.Fatalf("dad query should not return mom's private medical data, got: %+v", queryDad)
	}

	// 7. 检索命中测试：爸爸检索“净水器滤芯RO-800G”，应准确召回并置顶
	queryWater, err := svc.Query(ctx, dadID, "净水器滤芯RO-800G怎么复位", "all", 3)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}
	if len(queryWater) == 0 {
		t.Fatalf("expected to hit water purifier doc, got empty")
	}
	if queryWater[0].DocumentTitle != "美的净水器使用说明书" {
		t.Fatalf("unexpected doc title: %s", queryWater[0].DocumentTitle)
	}
}
