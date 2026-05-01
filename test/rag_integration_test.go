package test

import (
	"context"
	"testing"

	"myclouddrive-go/internal/agent/rag"
)

func TestChunkerSplit(t *testing.T) {
	c := rag.NewChunker(100, 20)
	doc := rag.Document{
		ID:       "doc1",
		FileName: "test.txt",
		Content:  "这是第一段测试文本。这里包含了足够多的内容，应该会被切分成多个chunk。当内容长度超过设定的chunk size时，切分器会自动进行切分。让我们看看效果如何。",
	}
	chunks := c.Split(doc)
	if len(chunks) == 0 {
		t.Fatal("expected at least 1 chunk")
	}
	if chunks[0].DocumentID != "doc1" {
		t.Errorf("expected docID=doc1, got %s", chunks[0].DocumentID)
	}
	if chunks[0].Text == "" {
		t.Error("chunk text should not be empty")
	}
}

func TestChunkerEmptyContent(t *testing.T) {
	c := rag.NewChunker(100, 20)
	doc := rag.Document{ID: "doc1", Content: ""}
	chunks := c.Split(doc)
	if len(chunks) != 0 {
		t.Errorf("expected 0 chunks for empty content, got %d", len(chunks))
	}
}

func TestNoopEmbedder(t *testing.T) {
	e := rag.NewNoopEmbedder(768)
	emb, err := e.Embed(context.Background(), "hello world")
	if err != nil {
		t.Fatalf("embed failed: %v", err)
	}
	if len(emb) != 768 {
		t.Errorf("expected 768 dims, got %d", len(emb))
	}

	embs, err := e.BatchEmbed(context.Background(), []string{"a", "b", "c"})
	if err != nil {
		t.Fatalf("batch embed failed: %v", err)
	}
	if len(embs) != 3 {
		t.Errorf("expected 3 embeddings, got %d", len(embs))
	}
}

func TestRetrieverKeywordSearch(t *testing.T) {
	e := rag.NewNoopEmbedder(16)
	r := rag.NewRetriever(e)
	chunks := []rag.Chunk{
		{ID: "c1", DocumentID: "d1", Text: "配置文件在哪里", Metadata: map[string]string{"source": "file1"}},
		{ID: "c2", DocumentID: "d2", Text: "数据库连接配置", Metadata: map[string]string{"source": "file2"}},
		{ID: "c3", DocumentID: "d3", Text: "分享链接管理", Metadata: map[string]string{"source": "file3"}},
	}
	embeddings := make([]rag.Embedding, 3)
	for i := 0; i < 3; i++ {
		embeddings[i] = make(rag.Embedding, 16)
		for j := range embeddings[i] {
			embeddings[i][j] = float32(i+j) * 0.01
		}
	}
	r.Index(chunks, embeddings)

	results, err := r.Search(context.Background(), rag.HybridQuery{
		Query:        "配置",
		TopK:         3,
		VectorWeight: 0.5,
	})
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}
	if len(results) == 0 {
		t.Error("expected results for '配置' keyword")
	}
}

func TestCosineSimilarity(t *testing.T) {
	a := rag.Embedding{1, 0, 0}
	b := rag.Embedding{1, 0, 0}
	sim := cosineSimilarityHelper(a, b)
	if sim < 0.99 {
		t.Errorf("expected similarity ~1.0, got %f", sim)
	}

	c := rag.Embedding{0, 1, 0}
	sim = cosineSimilarityHelper(a, c)
	if sim > 0.01 {
		t.Errorf("expected similarity ~0.0, got %f", sim)
	}
}

func cosineSimilarityHelper(a, b rag.Embedding) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dot, normA, normB float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		normA += float64(a[i]) * float64(a[i])
		normB += float64(b[i]) * float64(b[i])
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return dot / (normA * normB)
}

func TestRAGIndexer(t *testing.T) {
	e := rag.NewNoopEmbedder(16)
	r := rag.NewRetriever(e)
	c := rag.NewChunker(512, 64)
	idx := rag.NewIndexer(c, e, r)

	docs := []rag.Document{
		{ID: "d1", FileName: "config.md", Content: "这是一篇关于系统配置的文档，包含数据库配置、缓存配置和应用配置的详细说明。", FileType: "md"},
		{ID: "d2", FileName: "share_guide.md", Content: "分享功能使用指南：如何创建分享链接、设置提取码和过期时间。", FileType: "md"},
	}
	err := idx.IndexDocuments(context.Background(), docs)
	if err != nil {
		t.Fatalf("index failed: %v", err)
	}

	results, err := r.Search(context.Background(), rag.HybridQuery{
		Query:        "配置",
		TopK:         3,
		VectorWeight: 0.5,
	})
	if err != nil {
		t.Fatalf("search after index failed: %v", err)
	}
	if len(results) == 0 {
		t.Error("expected search results after indexing")
	}
}
