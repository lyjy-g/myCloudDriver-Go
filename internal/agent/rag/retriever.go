package rag

import (
	"context"
	"math"
	"sort"
	"strings"
	"sync"
)

type namespaceIndex struct {
	chunks     []Chunk
	embeddings map[string]Embedding
}

// Retriever 混合检索器（按 namespace 隔离索引）。
type Retriever struct {
	mu         sync.RWMutex
	namespaces map[string]*namespaceIndex
	embedder   Embedder
}

func NewRetriever(embedder Embedder) *Retriever {
	return &Retriever{
		namespaces: make(map[string]*namespaceIndex),
		embedder:   embedder,
	}
}

func normalizeNamespace(ns string) string {
	v := strings.TrimSpace(ns)
	if v == "" {
		return "__default__"
	}
	return v
}

// UpsertNamespace 覆盖写入一个 namespace 的全部 chunks 索引。
func (r *Retriever) UpsertNamespace(namespace string, chunks []Chunk, embeddings []Embedding) {
	ns := normalizeNamespace(namespace)
	r.mu.Lock()
	defer r.mu.Unlock()
	idx := &namespaceIndex{
		chunks:     make([]Chunk, 0, len(chunks)),
		embeddings: make(map[string]Embedding, len(chunks)),
	}
	for i, chunk := range chunks {
		idx.chunks = append(idx.chunks, chunk)
		if i < len(embeddings) {
			idx.embeddings[chunk.ID] = embeddings[i]
		}
	}
	r.namespaces[ns] = idx
}

// Search 执行混合检索：keyword + vector，RRF 融合。
func (r *Retriever) Search(ctx context.Context, query HybridQuery) ([]SearchResult, error) {
	topK := query.TopK
	if topK <= 0 {
		topK = 10
	}
	ns := normalizeNamespace(query.Namespace)
	r.mu.RLock()
	idx := r.namespaces[ns]
	r.mu.RUnlock()
	if idx == nil || len(idx.chunks) == 0 {
		return []SearchResult{}, nil
	}

	kw := keywordSearch(idx.chunks, query.Query, topK*3)
	vec := r.vectorSearch(ctx, idx, query.Query, topK*3)
	merged := rrfMerge(kw, vec, query.KeywordBoost)
	sort.SliceStable(merged, func(i, j int) bool { return merged[i].Score > merged[j].Score })
	if len(merged) > topK {
		merged = merged[:topK]
	}
	return merged, nil
}

func keywordSearch(chunks []Chunk, query string, k int) []SearchResult {
	terms := strings.Fields(strings.ToLower(strings.TrimSpace(query)))
	if len(terms) == 0 {
		return nil
	}
	results := make([]SearchResult, 0)
	for _, chunk := range chunks {
		text := strings.ToLower(chunk.Text)
		hit := 0
		for _, term := range terms {
			if strings.Contains(text, term) {
				hit++
			}
		}
		if hit == 0 {
			continue
		}
		results = append(results, SearchResult{
			ChunkID:    chunk.ID,
			DocumentID: chunk.DocumentID,
			Text:       chunk.Text,
			Score:      float64(hit) / float64(len(terms)),
			Source:     "keyword",
			Metadata:   chunk.Metadata,
		})
	}
	sort.SliceStable(results, func(i, j int) bool { return results[i].Score > results[j].Score })
	if len(results) > k {
		results = results[:k]
	}
	return results
}

func (r *Retriever) vectorSearch(ctx context.Context, idx *namespaceIndex, query string, k int) []SearchResult {
	if r.embedder == nil || idx == nil {
		return nil
	}
	vec, err := r.embedder.Embed(ctx, query)
	if err != nil {
		return nil
	}
	results := make([]SearchResult, 0)
	for _, chunk := range idx.chunks {
		emb, ok := idx.embeddings[chunk.ID]
		if !ok {
			continue
		}
		sim := cosineSimilarity(vec, emb)
		if sim <= 0 {
			continue
		}
		results = append(results, SearchResult{
			ChunkID:    chunk.ID,
			DocumentID: chunk.DocumentID,
			Text:       chunk.Text,
			Score:      sim,
			Source:     "vector",
			Metadata:   chunk.Metadata,
		})
	}
	sort.SliceStable(results, func(i, j int) bool { return results[i].Score > results[j].Score })
	if len(results) > k {
		results = results[:k]
	}
	return results
}

// rrfMerge 用 Reciprocal Rank Fusion 融合 keyword/vector 排名。
func rrfMerge(keyword, vector []SearchResult, keywordBoost float64) []SearchResult {
	const k = 60.0
	if keywordBoost <= 0 {
		keywordBoost = 1.0
	}
	type state struct {
		r     SearchResult
		score float64
	}
	merged := make(map[string]*state)
	for rank, r := range keyword {
		s := keywordBoost * (1.0 / (k + float64(rank+1)))
		cur, ok := merged[r.ChunkID]
		if !ok {
			cp := r
			merged[r.ChunkID] = &state{r: cp, score: s}
			continue
		}
		cur.score += s
	}
	for rank, r := range vector {
		s := 1.0 / (k + float64(rank+1))
		cur, ok := merged[r.ChunkID]
		if !ok {
			cp := r
			merged[r.ChunkID] = &state{r: cp, score: s}
			continue
		}
		cur.score += s
		cur.r.Source = "hybrid"
	}
	out := make([]SearchResult, 0, len(merged))
	for _, v := range merged {
		v.r.Score = v.score
		out = append(out, v.r)
	}
	return out
}

func cosineSimilarity(a, b Embedding) float64 {
	if len(a) == 0 || len(a) != len(b) {
		return 0
	}
	var dot, an, bn float64
	for i := range a {
		af := float64(a[i])
		bf := float64(b[i])
		dot += af * bf
		an += af * af
		bn += bf * bf
	}
	if an == 0 || bn == 0 {
		return 0
	}
	return dot / (math.Sqrt(an) * math.Sqrt(bn))
}
