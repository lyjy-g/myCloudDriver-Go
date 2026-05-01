package rag

import (
	"context"
	"math"
	"sort"
	"strings"
	"sync"
)

// Retriever 混合检索器。
type Retriever struct {
	mu         sync.RWMutex
	chunks     []Chunk
	embeddings map[string]Embedding // chunkID → embedding
	embedder   Embedder
}

func NewRetriever(embedder Embedder) *Retriever {
	return &Retriever{
		chunks:     make([]Chunk, 0),
		embeddings: make(map[string]Embedding),
		embedder:   embedder,
	}
}

// Index 索引 chunks（写入内存索引）。
func (r *Retriever) Index(chunks []Chunk, embeddings []Embedding) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i, chunk := range chunks {
		r.chunks = append(r.chunks, chunk)
		if i < len(embeddings) {
			r.embeddings[chunk.ID] = embeddings[i]
		}
	}
}

// Search 执行混合检索：keyword + vector 双路召回，结果合并去重。
func (r *Retriever) Search(ctx context.Context, query HybridQuery) ([]SearchResult, error) {
	topK := query.TopK
	if topK <= 0 {
		topK = 10
	}
	vectorWeight := query.VectorWeight
	if vectorWeight <= 0 {
		vectorWeight = 0.5
	}
	// 双路召回
	kwResults := r.keywordSearch(query.Query, topK*2)
	vecResults := r.vectorSearch(ctx, query.Query, topK*2)

	// 合并去重
	merged := mergeResults(kwResults, vecResults, vectorWeight, query.KeywordBoost)
	// 重排
	sort.SliceStable(merged, func(i, j int) bool { return merged[i].Score > merged[j].Score })
	if len(merged) > topK {
		merged = merged[:topK]
	}
	return merged, nil
}

func (r *Retriever) keywordSearch(query string, k int) []SearchResult {
	r.mu.RLock()
	defer r.mu.RUnlock()
	terms := strings.Fields(strings.ToLower(query))
	results := make([]SearchResult, 0)
	for _, chunk := range r.chunks {
		text := strings.ToLower(chunk.Text)
		score := 0.0
		for _, term := range terms {
			if strings.Contains(text, term) {
				score += 1.0
			}
		}
		if score > 0 {
			results = append(results, SearchResult{
				ChunkID:    chunk.ID,
				DocumentID: chunk.DocumentID,
				Text:       chunk.Text,
				Score:      score / float64(len(terms)),
				Source:     "keyword",
				Metadata:   chunk.Metadata,
			})
		}
	}
	sort.SliceStable(results, func(i, j int) bool { return results[i].Score > results[j].Score })
	if len(results) > k {
		results = results[:k]
	}
	return results
}

func (r *Retriever) vectorSearch(ctx context.Context, query string, k int) []SearchResult {
	if r.embedder == nil {
		return nil
	}
	queryVec, err := r.embedder.Embed(ctx, query)
	if err != nil {
		return nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	type scored struct {
		result SearchResult
		score  float64
	}
	items := make([]scored, 0)
	for _, chunk := range r.chunks {
		emb, ok := r.embeddings[chunk.ID]
		if !ok {
			continue
		}
		sim := cosineSimilarity(queryVec, emb)
		if sim > 0.3 {
			items = append(items, scored{
				result: SearchResult{
					ChunkID:    chunk.ID,
					DocumentID: chunk.DocumentID,
					Text:       chunk.Text,
					Score:      sim,
					Source:     "vector",
					Metadata:   chunk.Metadata,
				},
				score: sim,
			})
		}
	}
	sort.SliceStable(items, func(i, j int) bool { return items[i].score > items[j].score })
	results := make([]SearchResult, 0, len(items))
	for i, item := range items {
		if i >= k {
			break
		}
		results = append(results, item.result)
	}
	return results
}

func mergeResults(kw, vec []SearchResult, vectorWeight, keywordBoost float64) []SearchResult {
	seen := make(map[string]float64)
	for _, r := range kw {
		boost := r.Score
		if keywordBoost > 0 {
			boost *= keywordBoost
		}
		seen[r.ChunkID] = boost * (1 - vectorWeight)
	}
	for _, r := range vec {
		existing := seen[r.ChunkID]
		if existing > 0 {
			// 双路命中加分
			seen[r.ChunkID] = existing + r.Score*vectorWeight*1.5
		} else {
			seen[r.ChunkID] = r.Score * vectorWeight
		}
	}
	results := make([]SearchResult, 0)
	for id, score := range seen {
		// 需要找到对应的 chunk 元信息，这里从两路中取
		chunkID := id
		var text string
		var docID string
		var source string
		var meta map[string]string
		for _, r := range kw {
			if r.ChunkID == chunkID {
				text = r.Text
				docID = r.DocumentID
				source = r.Source
				meta = r.Metadata
				break
			}
		}
		if text == "" {
			for _, r := range vec {
				if r.ChunkID == chunkID {
					text = r.Text
					docID = r.DocumentID
					source = r.Source
					meta = r.Metadata
					break
				}
			}
		}
		if text == "" {
			continue
		}
		if source != "keyword" {
			source = "hybrid"
		}
		results = append(results, SearchResult{
			ChunkID:    chunkID,
			DocumentID: docID,
			Text:       text,
			Score:      score,
			Source:     source,
			Metadata:   meta,
		})
	}
	return results
}

func cosineSimilarity(a, b Embedding) float64 {
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
	return dot / (math.Sqrt(normA) * math.Sqrt(normB))
}
