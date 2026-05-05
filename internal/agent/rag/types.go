package rag

import "time"

// Document 表示一个待索引的文档。
type Document struct {
	ID        string
	FileName  string
	FileType  string
	Content   string
	Size      int64
	SourceKey string
	CreatedAt time.Time
}

// Chunk 表示文档切分后的一个片段。
type Chunk struct {
	ID         string
	DocumentID string
	Index      int
	Text       string
	StartByte  int
	EndByte    int
	Metadata   map[string]string
}

// Embedding 表示向量。
type Embedding []float32

// SearchResult 表示单条检索结果。
type SearchResult struct {
	ChunkID    string
	DocumentID string
	Text       string
	Score      float64
	Source     string // "keyword" / "vector"
	Metadata   map[string]string
}

// HybridQuery 混合查询参数。
type HybridQuery struct {
	Namespace    string
	Query        string
	TopK         int
	VectorWeight float64
	KeywordBoost float64
}
