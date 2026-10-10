package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// KnowledgeDocument 个人与家庭知识库文档（说明书/保单/合同/健康档案/生活备忘/菜谱）。
type KnowledgeDocument struct{ ent.Schema }

func (KnowledgeDocument) Mixin() []ent.Mixin { return []ent.Mixin{UUIDMixin{}, TimeMixin{}} }

func (KnowledgeDocument) Fields() []ent.Field {
	return []ent.Field{
		field.String("title").Comment("文档标题"),
		field.Enum("category").Values(
			"manual", "insurance", "contract", "health", "memo", "recipe",
		).Default("manual").Comment("文档分类"),
		field.Enum("visibility").Values(
			"family", "private",
		).Default("family").Comment("可见范围：family(家庭共享) | private(个人私密)"),
		field.Text("content").Comment("文档完整正文内容 (Markdown/纯文本)"),
		field.String("file_path").Optional().Comment("原始文件存储相对路径（如PDF等）"),
		field.Int64("file_size_bytes").Default(0).Comment("文件字节大小"),
		field.Int("chunk_count").Default(0).Comment("切片总数"),
		field.Enum("status").Values(
			"indexed", "processing", "failed",
		).Default("indexed").Comment("处理与索引状态"),
	}
}

func (KnowledgeDocument) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("member", Member.Type).Ref("documents").Unique().Required().Comment("文档拥有者/上传者"),
		edge.To("chunks", KnowledgeChunk.Type),
	}
}

func (KnowledgeDocument) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("visibility"),
		index.Fields("category"),
		index.Fields("created_at"),
	}
}

// KnowledgeChunk 知识切片（用于混合检索 RAG 与关键词精准定位）。
type KnowledgeChunk struct{ ent.Schema }

func (KnowledgeChunk) Mixin() []ent.Mixin { return []ent.Mixin{UUIDMixin{}} }

func (KnowledgeChunk) Fields() []ent.Field {
	return []ent.Field{
		field.Int("chunk_index").Default(0).Comment("切片在文档内的顺序编号 (0, 1, 2...)"),
		field.Text("content").Comment("切片文本片段 (~400字符)"),
		field.Int("token_count").Default(0).Comment("Token 估算数量"),
		field.Bytes("embedding").Optional().Comment("向量 Float32 序列化二进制"),
		field.Time("created_at").Default(time.Now).Immutable(),
	}
}

func (KnowledgeChunk) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("document", KnowledgeDocument.Type).Ref("chunks").Unique().Required(),
	}
}

func (KnowledgeChunk) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("chunk_index"),
	}
}
