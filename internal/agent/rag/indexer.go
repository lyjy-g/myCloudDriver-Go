package rag

import (
	"context"
	"fmt"
)

// Indexer 负责写入 namespace 级索引。
type Indexer struct {
	embedder  Embedder
	retriever *Retriever
}

func NewIndexer(embedder Embedder, retriever *Retriever) *Indexer {
	return &Indexer{embedder: embedder, retriever: retriever}
}

// IndexNamespace 覆盖索引一个 namespace 的全部 chunks。
func (idx *Indexer) IndexNamespace(ctx context.Context, namespace string, chunks []Chunk) error {
	if idx == nil || idx.embedder == nil || idx.retriever == nil {
		return fmt.Errorf("rag indexer unavailable")
	}
	if len(chunks) == 0 {
		idx.retriever.UpsertNamespace(namespace, []Chunk{}, []Embedding{})
		return nil
	}
	texts := make([]string, 0, len(chunks))
	for _, c := range chunks {
		texts = append(texts, c.Text)
	}
	vectors, err := idx.embedder.BatchEmbed(ctx, texts)
	if err != nil {
		return fmt.Errorf("embed chunks failed: %w", err)
	}
	idx.retriever.UpsertNamespace(namespace, chunks, vectors)
	return nil
}
