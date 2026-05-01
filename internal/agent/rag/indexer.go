package rag

import (
	"context"
	"fmt"
)

// Indexer 负责将文档写入索引。
type Indexer struct {
	chunker   *Chunker
	embedder  Embedder
	retriever *Retriever
}

func NewIndexer(chunker *Chunker, embedder Embedder, retriever *Retriever) *Indexer {
	return &Indexer{chunker: chunker, embedder: embedder, retriever: retriever}
}

// IndexDocuments 批量索引文档。
func (idx *Indexer) IndexDocuments(ctx context.Context, docs []Document) error {
	if len(docs) == 0 {
		return fmt.Errorf("no documents to index")
	}
	allChunks := make([]Chunk, 0)
	allTexts := make([]string, 0)
	for _, doc := range docs {
		chunks := idx.chunker.Split(doc)
		for _, chunk := range chunks {
			allChunks = append(allChunks, chunk)
			allTexts = append(allTexts, chunk.Text)
		}
	}
	if len(allChunks) == 0 {
		return fmt.Errorf("no chunks produced from %d documents", len(docs))
	}
	embeddings, err := idx.embedder.BatchEmbed(ctx, allTexts)
	if err != nil {
		return fmt.Errorf("embed chunks failed: %w", err)
	}
	idx.retriever.Index(allChunks, embeddings)
	return nil
}

// IndexDocument 索引单个文档。
func (idx *Indexer) IndexDocument(ctx context.Context, doc Document) error {
	return idx.IndexDocuments(ctx, []Document{doc})
}
