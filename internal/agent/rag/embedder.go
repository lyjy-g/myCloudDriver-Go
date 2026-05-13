package rag

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Embedder 定义向量化接口。
type Embedder interface {
	Embed(ctx context.Context, text string) (Embedding, error)
	BatchEmbed(ctx context.Context, texts []string) ([]Embedding, error)
	Dims() int
}

// NoopEmbedder 用于开发和测试。
type NoopEmbedder struct{ dims int }

func NewNoopEmbedder(dims int) *NoopEmbedder {
	if dims <= 0 {
		dims = 768
	}
	return &NoopEmbedder{dims: dims}
}

func (e *NoopEmbedder) Embed(_ context.Context, text string) (Embedding, error) {
	vec := make(Embedding, e.dims)
	for i := range vec {
		vec[i] = float32((len(text)+i)%17) * 0.01
	}
	return vec, nil
}

func (e *NoopEmbedder) BatchEmbed(ctx context.Context, texts []string) ([]Embedding, error) {
	out := make([]Embedding, 0, len(texts))
	for _, t := range texts {
		v, err := e.Embed(ctx, t)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}

func (e *NoopEmbedder) Dims() int { return e.dims }

// HTTPEmbedder 通过 HTTP API 调 embedding，支持 dashscope/openai 兼容入参。
type HTTPEmbedder struct {
	provider string
	baseURL  string
	apiKey   string
	model    string
	dims     int
	client   *http.Client
}

func NewHTTPEmbedder(provider, baseURL, apiKey, model string, dims int, timeout time.Duration) *HTTPEmbedder {
	if dims <= 0 {
		dims = 1024
	}
	if timeout <= 0 {
		timeout = 8 * time.Second
	}
	return &HTTPEmbedder{
		provider: strings.ToLower(strings.TrimSpace(provider)),
		baseURL:  strings.TrimSpace(baseURL),
		apiKey:   strings.TrimSpace(apiKey),
		model:    strings.TrimSpace(model),
		dims:     dims,
		client:   &http.Client{Timeout: timeout},
	}
}

func (e *HTTPEmbedder) Dims() int { return e.dims }

func (e *HTTPEmbedder) Embed(ctx context.Context, text string) (Embedding, error) {
	vectors, err := e.BatchEmbed(ctx, []string{text})
	if err != nil {
		return nil, err
	}
	if len(vectors) == 0 {
		return nil, fmt.Errorf("embedding error: empty result")
	}
	return vectors[0], nil
}

func (e *HTTPEmbedder) BatchEmbed(ctx context.Context, texts []string) ([]Embedding, error) {
	if len(texts) == 0 {
		return []Embedding{}, nil
	}
	if e.baseURL == "" {
		return nil, fmt.Errorf("embedding config error: base_url is empty")
	}
	if e.apiKey == "" {
		return nil, fmt.Errorf("embedding config error: api_key is empty")
	}
	if e.model == "" {
		return nil, fmt.Errorf("embedding config error: model is empty")
	}

	body, err := e.buildRequestBody(texts)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.baseURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("embedding request build error: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+e.apiKey)

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("embedding network error: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("embedding http error: status=%d body=%s", resp.StatusCode, string(raw))
	}

	vectors, parseErr := parseEmbeddingResponse(raw)
	if parseErr != nil {
		return nil, fmt.Errorf("embedding parse error: %w", parseErr)
	}
	if len(vectors) != len(texts) {
		return nil, fmt.Errorf("embedding result mismatch: want=%d got=%d", len(texts), len(vectors))
	}
	return vectors, nil
}

func (e *HTTPEmbedder) buildRequestBody(texts []string) ([]byte, error) {
	// dashscope: {"model":"text-embedding-v4","input":{"texts":[...]}}
	if e.provider == "dashscope" {
		payload := map[string]any{
			"model": e.model,
			"input": map[string]any{"texts": texts},
		}
		return json.Marshal(payload)
	}
	// openai-like fallback: {"model":"..","input":[...]}
	payload := map[string]any{
		"model": e.model,
		"input": texts,
	}
	return json.Marshal(payload)
}

func parseEmbeddingResponse(raw []byte) ([]Embedding, error) {
	// dashscope response structure
	var ds struct {
		Output struct {
			Embeddings []struct {
				Embedding []float32 `json:"embedding"`
				TextIndex int       `json:"text_index"`
			} `json:"embeddings"`
		} `json:"output"`
	}
	if err := json.Unmarshal(raw, &ds); err == nil && len(ds.Output.Embeddings) > 0 {
		out := make([]Embedding, len(ds.Output.Embeddings))
		for i, it := range ds.Output.Embeddings {
			out[i] = Embedding(it.Embedding)
		}
		return out, nil
	}

	// openai-like response structure
	var oa struct {
		Data []struct {
			Embedding []float32 `json:"embedding"`
			Index     int       `json:"index"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &oa); err == nil && len(oa.Data) > 0 {
		out := make([]Embedding, len(oa.Data))
		for i, it := range oa.Data {
			out[i] = Embedding(it.Embedding)
		}
		return out, nil
	}
	return nil, fmt.Errorf("unknown embedding response format")
}
