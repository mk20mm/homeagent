package repo

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/mk20mm/homeagent/internal/apperr"
	domknow "github.com/mk20mm/homeagent/internal/domain/knowledge"
	"github.com/mk20mm/homeagent/internal/store/ent"
	"github.com/mk20mm/homeagent/internal/store/ent/knowledgechunk"
	"github.com/mk20mm/homeagent/internal/store/ent/knowledgedocument"
	"github.com/mk20mm/homeagent/internal/store/ent/member"
	"github.com/mk20mm/homeagent/internal/store/ent/predicate"
)

var _ domknow.Repo = (*Store)(nil)

// CreateDocument 创建知识库文档并批量写入切片
func (s *Store) CreateDocument(ctx context.Context, doc domknow.Document, chunks []domknow.Chunk) (domknow.Document, error) {
	tx, err := s.db.Tx(ctx)
	if err != nil {
		return domknow.Document{}, apperr.New(apperr.CodeInternal, "开启事务失败", err)
	}
	defer func() {
		if r := recover(); r != nil {
			_ = tx.Rollback()
			panic(r)
		}
	}()

	memUUID := toUUID(doc.MemberID)
	// 若无具体 memberID，默认使用库中首个成员
	if memUUID == uuid.Nil {
		firstMember, err := tx.Member.Query().First(ctx)
		if err == nil && firstMember != nil {
			memUUID = firstMember.ID
			doc.MemberID = memUUID.String()
			doc.AuthorName = firstMember.Name
		}
	} else {
		m, err := tx.Member.Get(ctx, memUUID)
		if err == nil && m != nil {
			doc.AuthorName = m.Name
		}
	}

	createDoc := tx.KnowledgeDocument.Create().
		SetTitle(doc.Title).
		SetCategory(knowledgedocument.Category(doc.Category)).
		SetVisibility(knowledgedocument.Visibility(doc.Visibility)).
		SetContent(doc.Content).
		SetFileSizeBytes(doc.FileSizeBytes).
		SetChunkCount(len(chunks)).
		SetStatus(knowledgedocument.Status(doc.Status)).
		SetMemberID(memUUID)

	if doc.FilePath != "" {
		createDoc.SetFilePath(doc.FilePath)
	}

	savedDoc, err := createDoc.Save(ctx)
	if err != nil {
		_ = tx.Rollback()
		return domknow.Document{}, apperr.New(apperr.CodeInternal, "创建知识库文档失败", err)
	}

	// 批量写入切片
	if len(chunks) > 0 {
		bulk := make([]*ent.KnowledgeChunkCreate, len(chunks))
		for i, c := range chunks {
			bulk[i] = tx.KnowledgeChunk.Create().
				SetDocument(savedDoc).
				SetChunkIndex(c.ChunkIndex).
				SetContent(c.Content).
				SetTokenCount(c.TokenCount)
			if len(c.Embedding) > 0 {
				bulk[i].SetEmbedding(c.Embedding)
			}
		}
		if _, err := tx.KnowledgeChunk.CreateBulk(bulk...).Save(ctx); err != nil {
			_ = tx.Rollback()
			return domknow.Document{}, apperr.New(apperr.CodeInternal, "保存知识切片失败", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return domknow.Document{}, apperr.New(apperr.CodeInternal, "提交事务失败", err)
	}

	return domknow.Document{
		ID:            savedDoc.ID.String(),
		Title:         savedDoc.Title,
		Category:      string(savedDoc.Category),
		Visibility:    string(savedDoc.Visibility),
		Content:       savedDoc.Content,
		FilePath:      savedDoc.FilePath,
		FileSizeBytes: savedDoc.FileSizeBytes,
		ChunkCount:    savedDoc.ChunkCount,
		MemberID:      memUUID.String(),
		AuthorName:    doc.AuthorName,
		Status:        string(savedDoc.Status),
		CreatedAt:     savedDoc.CreatedAt,
		UpdatedAt:     savedDoc.UpdatedAt,
	}, nil
}

// GetDocument 获取文档详情
func (s *Store) GetDocument(ctx context.Context, id string) (domknow.Document, error) {
	d, err := s.db.KnowledgeDocument.Query().
		Where(knowledgedocument.IDEQ(toUUID(id)), knowledgedocument.DeletedAtIsNil()).
		WithMember().
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return domknow.Document{}, apperr.New(apperr.CodeNotFound, "知识库文档不存在", err)
		}
		return domknow.Document{}, apperr.New(apperr.CodeInternal, "查询知识库文档失败", err)
	}

	authorName := "家庭成员"
	memberIDStr := ""
	if d.Edges.Member != nil {
		authorName = d.Edges.Member.Name
		memberIDStr = d.Edges.Member.ID.String()
	}

	return domknow.Document{
		ID:            d.ID.String(),
		Title:         d.Title,
		Category:      string(d.Category),
		Visibility:    string(d.Visibility),
		Content:       d.Content,
		FilePath:      d.FilePath,
		FileSizeBytes: d.FileSizeBytes,
		ChunkCount:    d.ChunkCount,
		MemberID:      memberIDStr,
		AuthorName:    authorName,
		Status:        string(d.Status),
		CreatedAt:     d.CreatedAt,
		UpdatedAt:     d.UpdatedAt,
	}, nil
}

// ListDocuments 浏览知识库文档列表（严格执行空间与权限过滤）
func (s *Store) ListDocuments(ctx context.Context, memberID string, visibility string, category string, keyword string) ([]domknow.Document, error) {
	q := s.db.KnowledgeDocument.Query().Where(knowledgedocument.DeletedAtIsNil())

	memUUID := toUUID(memberID)

	// 空间/可见性过滤（ADR-005 权限双保险）
	switch visibility {
	case domknow.VisibilityPrivate:
		q = q.Where(
			knowledgedocument.VisibilityEQ(knowledgedocument.VisibilityPrivate),
			knowledgedocument.HasMemberWith(member.IDEQ(memUUID)),
		)
	case domknow.VisibilityFamily:
		q = q.Where(knowledgedocument.VisibilityEQ(knowledgedocument.VisibilityFamily))
	default: // all: 家庭共享 + 自己的私密
		q = q.Where(
			knowledgedocument.Or(
				knowledgedocument.VisibilityEQ(knowledgedocument.VisibilityFamily),
				knowledgedocument.And(
					knowledgedocument.VisibilityEQ(knowledgedocument.VisibilityPrivate),
					knowledgedocument.HasMemberWith(member.IDEQ(memUUID)),
				),
			),
		)
	}

	// 分类过滤
	if category != "" && category != "all" {
		q = q.Where(knowledgedocument.CategoryEQ(knowledgedocument.Category(category)))
	}

	// 关键词模糊匹配
	if keyword != "" {
		q = q.Where(
			knowledgedocument.Or(
				knowledgedocument.TitleContainsFold(keyword),
				knowledgedocument.ContentContainsFold(keyword),
			),
		)
	}

	docs, err := q.Order(ent.Desc(knowledgedocument.FieldCreatedAt)).WithMember().All(ctx)
	if err != nil {
		return nil, apperr.New(apperr.CodeInternal, "查询知识库文档列表失败", err)
	}

	results := make([]domknow.Document, len(docs))
	for i, d := range docs {
		authorName := "家庭成员"
		memberIDStr := ""
		if d.Edges.Member != nil {
			authorName = d.Edges.Member.Name
			memberIDStr = d.Edges.Member.ID.String()
		}
		results[i] = domknow.Document{
			ID:            d.ID.String(),
			Title:         d.Title,
			Category:      string(d.Category),
			Visibility:    string(d.Visibility),
			Content:       d.Content,
			FilePath:      d.FilePath,
			FileSizeBytes: d.FileSizeBytes,
			ChunkCount:    d.ChunkCount,
			MemberID:      memberIDStr,
			AuthorName:    authorName,
			Status:        string(d.Status),
			CreatedAt:     d.CreatedAt,
			UpdatedAt:     d.UpdatedAt,
		}
	}
	return results, nil
}

// DeleteDocument 软删除文档（支持 24h 可撤销恢复）
func (s *Store) DeleteDocument(ctx context.Context, id string) error {
	now := time.Now()
	err := s.db.KnowledgeDocument.UpdateOneID(toUUID(id)).
		SetDeletedAt(now).
		Exec(ctx)
	if err != nil {
		return apperr.New(apperr.CodeInternal, "删除知识库文档失败", err)
	}
	return nil
}

// RestoreDocument 撤销恢复已删除文档
func (s *Store) RestoreDocument(ctx context.Context, id string) error {
	err := s.db.KnowledgeDocument.UpdateOneID(toUUID(id)).
		ClearDeletedAt().
		Exec(ctx)
	if err != nil {
		return apperr.New(apperr.CodeInternal, "恢复知识库文档失败", err)
	}
	return nil
}

// QueryChunks 检索符合权限条件的切片
func (s *Store) QueryChunks(ctx context.Context, memberID string, category string) ([]domknow.ChunkWithDoc, error) {
	memUUID := toUUID(memberID)

	var docPreds []predicate.KnowledgeDocument
	docPreds = append(docPreds, knowledgedocument.DeletedAtIsNil())

	// 权限过滤：只能检索家庭共享或本人的私密文档
	docPreds = append(docPreds, knowledgedocument.Or(
		knowledgedocument.VisibilityEQ(knowledgedocument.VisibilityFamily),
		knowledgedocument.And(
			knowledgedocument.VisibilityEQ(knowledgedocument.VisibilityPrivate),
			knowledgedocument.HasMemberWith(member.IDEQ(memUUID)),
		),
	))

	if category != "" && category != "all" {
		docPreds = append(docPreds, knowledgedocument.CategoryEQ(knowledgedocument.Category(category)))
	}

	chunks, err := s.db.KnowledgeChunk.Query().
		Where(knowledgechunk.HasDocumentWith(docPreds...)).
		WithDocument(func(dq *ent.KnowledgeDocumentQuery) {
			dq.WithMember()
		}).
		All(ctx)
	if err != nil {
		return nil, apperr.New(apperr.CodeInternal, "查询知识库切片失败", err)
	}

	results := make([]domknow.ChunkWithDoc, len(chunks))
	for i, c := range chunks {
		docTitle := "家庭文档"
		docCategory := "manual"
		docVisibility := domknow.VisibilityFamily
		docMemberID := ""
		if c.Edges.Document != nil {
			docTitle = c.Edges.Document.Title
			docCategory = string(c.Edges.Document.Category)
			docVisibility = string(c.Edges.Document.Visibility)
			if c.Edges.Document.Edges.Member != nil {
				docMemberID = c.Edges.Document.Edges.Member.ID.String()
			}
		}

		results[i] = domknow.ChunkWithDoc{
			Chunk: domknow.Chunk{
				ID:         c.ID.String(),
				DocumentID: c.Edges.Document.ID.String(),
				ChunkIndex: c.ChunkIndex,
				Content:    c.Content,
				TokenCount: c.TokenCount,
				Embedding:  c.Embedding,
				CreatedAt:  c.CreatedAt,
			},
			DocTitle:      docTitle,
			DocCategory:   docCategory,
			DocVisibility: docVisibility,
			DocMemberID:   docMemberID,
		}
	}
	return results, nil
}

// CountDocumentsBySpace 统计空间文档数量
func (s *Store) CountDocumentsBySpace(ctx context.Context, memberID string) (familyCount int, privateCount int, err error) {
	memUUID := toUUID(memberID)

	fCount, err := s.db.KnowledgeDocument.Query().
		Where(
			knowledgedocument.DeletedAtIsNil(),
			knowledgedocument.VisibilityEQ(knowledgedocument.VisibilityFamily),
		).Count(ctx)
	if err != nil {
		return 0, 0, apperr.New(apperr.CodeInternal, "统计共享知识库文档数失败", err)
	}

	pCount, err := s.db.KnowledgeDocument.Query().
		Where(
			knowledgedocument.DeletedAtIsNil(),
			knowledgedocument.VisibilityEQ(knowledgedocument.VisibilityPrivate),
			knowledgedocument.HasMemberWith(member.IDEQ(memUUID)),
		).Count(ctx)
	if err != nil {
		return 0, 0, apperr.New(apperr.CodeInternal, "统计私密知识库文档数失败", err)
	}

	return fCount, pCount, nil
}
