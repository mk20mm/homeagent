# 个人与家庭知识库深化设计（Family Knowledge Base Design）

> **模块编号**：M9（Family Second Brain & Knowledge Base · 混合检索 RAG）  
> **文档定位**：技术规格与架构设计说明书（Phase E 第三顺位）  
> **对齐标准**：AI-STD-003（架构标准）、AI-STD-004（上下文与有界检索）、ADR-003（SQLite 基石）、ADR-005（权限双保险）

---

## 1. 业务场景与解决的痛点

家庭生活中沉淀了大量私有文件与生活资产，**散落在各个抽屉、相册或微信收藏中，需要用时极难找到**：
1. **家电与设备说明书**：说明书纸质版易丢失；当遇到净水器报警、空气炸锅报错或扫地机配网失败时，需要秒级问出“净水器怎么手动冲洗滤芯”。
2. **家庭保单与合同档案**：“我们家给孩子买的医疗险免赔额是多少”、“租房合同什么时候到期、提前退租押金怎么算”。
3. **健康与就医备忘**：“宝宝上次打麻腮风疫苗是什么时候”、“老人的体检报告里低密度脂蛋白偏高要注意什么饮食”。
4. **家庭关键资产备忘**：水暖总阀门位置、电表户号、常用家庭 Wi-Fi 密码、装修建材色号与保修凭据。

通用大模型完全不具备这些私密上下文，需要一套**轻量级、零重型运维、具备严格家庭权限隔离**的私域知识库系统。

---

## 2. 混合检索架构（SQLite FTS5 + 嵌入向量）

为了严格遵循“单 VPS 自用、零复杂运维”的原则，**拒绝引入 Milvus/Chroma 等重量级独立向量服务**。我们采用纯 Go 友好、原生嵌入在 SQLite 内的**混合检索架构（Hybrid Search）**：

```mermaid
graph TD
    subgraph Ingestion [文档导入与分块流水线]
        Doc[上传文档: PDF/MD/TXT] --> Parser[文本解析与清洗]
        Parser --> Chunker[滑窗切片: 400字+50字重叠]
        Chunker --> Embedder[生成嵌入向量: 适配 OpenAI/bge-m3/nomic]
        Chunker --> FTS5Index[写入 SQLite FTS5 全文索引]
        Embedder --> VectorBlob[写入 SQLite Chunk 表: Blob 向量]
    end

    subgraph Retrieval [混合召回与融合 RRF]
        Query[用户提问: '净水器怎么复位'] --> FTSQuery[FTS5 关键词匹配得分 (精准数字/型号)]
        Query --> VecQuery[向量余弦相似度计算 (语义意图)]
        FTSQuery --> RRF[RRF 混合加权重排: Score = 1/(k+rank_fts) + 1/(k+rank_vec)]
        VecQuery --> RRF
        RRF --> PermFilter[确定性权限过滤: member_id 隔离]
        PermFilter --> TopK[精准 Top 3 片段注入 Agent 上下文]
    end
```

### 为什么选择 Hybrid FTS5 + Vector？
- **FTS5 专长**：型号、专有名词、编号（如 `RO-800G`、`13800138000`、`合同编号HT2024`）命中率高达 100%，纯向量模型在精确编号上极易失真。
- **Vector 专长**：解决口语化表达与模糊表述（如问“那个洗菜洗碗的机器怎么弄”能匹配到“洗碗机操作规程”）。
- **轻量可控**：家庭私域 Chunk 通常在数千级别以内，SQLite FTS5 配合 Go 内存余弦计算耗时小于 10ms，零运维负担。

---

## 3. 数据模型设计（Ent Schema 草案）

```go
// 1. KnowledgeDocument: 文档元数据
type KnowledgeDocument struct {
	ID          uuid.UUID
	Title       string    // 文档标题（如 "美的净水器使用说明书.pdf"）
	Category    string    // 分类: manual(说明书), insurance(保单), contract(合同), health(健康), memo(备忘)
	SourceType  string    // file / text_note / url
	FilePath    string    // 存储相对路径
	FileSizeBytes int64
	MemberID    uuid.UUID // 上传者 ID
	Visibility  string    // "family" (全家公开) 或 "private" (仅本人与配偶私密)
	Status      string    // indexed(已建索引) / processing / failed
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// 2. KnowledgeChunk: 切片与向量
type KnowledgeChunk struct {
	ID         uuid.UUID
	DocumentID uuid.UUID // 关联文档外键
	ChunkIndex int       // 序号 (0, 1, 2...)
	Content    string    // 文本正文片段 (~400 字符)
	TokenCount int       // Token 数量估算
	Embedding  []byte    // 向量 Float32 序列化后的二进制字节流
	CreatedAt  time.Time
}
```

---

## 4. 工具契约（Tool Spec）

### 4.1 工具定义与元数据
- **工具名称**：`query_knowledge`
- **目的（Purpose）**：根据用户问题检索家庭内部说明书、合同、保单、健康或私有备忘录资料。
- **何时使用**：涉及家庭专属物品、设备操作、私人物品存放位置、保单条款或家庭特定约定。
- **何时禁止使用**：用户询问公开网络信息（如“明天天气”、“苹果发布会”）时，应调用 `web_search` 而非本工具。
- **安全等级**：**A1（只读工具）**。

### 4.2 参数契约（JSON Schema）

```json
{
  "name": "query_knowledge",
  "description": "检索家庭私域知识库（家电说明书、合同保单条款、家庭健康记录与重要备忘）",
  "parameters": {
    "type": "object",
    "required": ["query"],
    "properties": {
      "query": {
        "type": "string",
        "description": "检索关键词或问题句"
      },
      "category": {
        "type": "string",
        "enum": ["all", "manual", "insurance", "contract", "health", "memo"],
        "default": "all",
        "description": "限定分类，可大幅提高匹配精准度"
      },
      "top_k": {
        "type": "integer",
        "minimum": 1,
        "maximum": 5,
        "default": 3,
        "description": "召回最多相关的文本片段数"
      }
    }
  }
}
```

---

## 5. 权限双保险与数据隔离机制（ADR-005）

家庭知识库具有极高的隐私敏感度（如个人体检病历、私人财务合同），**绝不允许越权访问**：

1. **源头权限过滤（SQL 强制绑定）**：
   无论 LLM 传入何种参数，检索底层在执行查询时，强制拼接鉴权约束：
   ```sql
   WHERE (visibility = 'family' OR (visibility = 'private' AND member_id = :current_member_id))
   ```
   **孩子或访客角色**甚至可直接在工具层禁用 `insurance` 和 `contract` 分类的查询。
2. **上下文有界注入（Strict Token Budget）**：
   - 召回的知识切片总注入量限制在 1200 Token 以内；
   - 携带元数据头部：`[家庭知识库参考: 来源《美的净水器说明书》第3节 (已验证有效)]`，清晰界定私域知识与模型通用常识。
3. **写操作必须可撤销（ADR-004）**：
   - 用户删除文档或更新说明书，必须进入 `undo_log`，24 小时内可通过对话撤销误删。
