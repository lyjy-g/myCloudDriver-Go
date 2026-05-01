package rag

import "context"

// Embedder 定义向量化接口。
type Embedder interface {
	// Embed 将文本转为向量。
	Embed(ctx context.Context, text string) (Embedding, error)
	// BatchEmbed 批量向量化。
	BatchEmbed(ctx context.Context, texts []string) ([]Embedding, error)
	// Dims 返回向量维度。
	Dims() int
}

// NoopEmbedder 用于开发和测试的空实现。
type NoopEmbedder struct{ dims int }

func NewNoopEmbedder(dims int) *NoopEmbedder {
	if dims <= 0 {
		dims = 768
	}
	return &NoopEmbedder{dims: dims}
}

func (e *NoopEmbedder) Embed(_ context.Context, text string) (Embedding, error) {
	vec := make(Embedding, e.dims)
	// 用文本长度做简单的伪向量，保证不同文本有不同向量
	for i := range vec {
		vec[i] = float32(len(text)%(i+3)) * 0.01
	}
	return vec, nil
}

func (e *NoopEmbedder) BatchEmbed(_ context.Context, texts []string) ([]Embedding, error) {
	result := make([]Embedding, 0, len(texts))
	for _, t := range texts {
		emb, err := e.Embed(nil, t)
		if err != nil {
			return nil, err
		}
		result = append(result, emb)
	}
	return result, nil
}

func (e *NoopEmbedder) Dims() int { return e.dims }

// DeepSeekEmbedder 使用 DeepSeek Embedding API 的实现桩。
type DeepSeekEmbedder struct {
	baseURL string
	apiKey  string
	model   string
	dims    int
}

func NewDeepSeekEmbedder(baseURL, apiKey, model string, dims int) *DeepSeekEmbedder {
	return &DeepSeekEmbedder{baseURL: baseURL, apiKey: apiKey, model: model, dims: dims}
}

func (e *DeepSeekEmbedder) Embed(ctx context.Context, text string) (Embedding, error) {
	// TODO: 调用 DeepSeek Embedding API
	// POST {baseURL}/embeddings with {model, input: [text]}
	return make(Embedding, e.dims), nil
}

func (e *DeepSeekEmbedder) BatchEmbed(ctx context.Context, texts []string) ([]Embedding, error) {
	// TODO: 批量调用
	result := make([]Embedding, 0, len(texts))
	for _, t := range texts {
		emb, err := e.Embed(ctx, t)
		if err != nil {
			return nil, err
		}
		result = append(result, emb)
	}
	return result, nil
}

func (e *DeepSeekEmbedder) Dims() int { return e.dims }
