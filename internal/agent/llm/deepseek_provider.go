package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

// DeepSeekProvider 是 DeepSeek Chat API 的最小封装。
type DeepSeekProvider struct {
	baseURL string
	apiKey  string
	model   string
	client  *http.Client
}

func NewDeepSeekProvider(baseURL, apiKey, model string, timeout time.Duration) *DeepSeekProvider {
	if strings.TrimSpace(apiKey) == "" {
		apiKey = strings.TrimSpace(os.Getenv("DEEPSEEK_API_KEY"))
	}
	if strings.TrimSpace(baseURL) == "" {
		baseURL = "https://api.deepseek.com"
	}
	if strings.TrimSpace(model) == "" {
		model = "deepseek-chat"
	}
	if timeout <= 0 {
		timeout = 8 * time.Second
	}
	return &DeepSeekProvider{baseURL: strings.TrimRight(baseURL, "/"), apiKey: apiKey, model: model, client: &http.Client{Timeout: timeout}}
}

func (p *DeepSeekProvider) Name() string  { return "deepseek" }
func (p *DeepSeekProvider) Model() string { return p.model }

func (p *DeepSeekProvider) DecideTools(ctx context.Context, query string) (Decision, error) {
	prompt := "你是网盘检索路由器，只能从工具白名单中选择: tool.file.list, tool.share.list, tool.share.records。" +
		"输出严格 JSON: {\"intent\":\"...\",\"tools\":[\"tool.file.list\"]}。" +
		"如果是文件检索优先 tool.file.list；分享列表用 tool.share.list；访问记录用 tool.share.records。用户问题: " + query
	content, err := p.chat(ctx, prompt)
	if err != nil {
		return Decision{}, err
	}
	start := strings.Index(content, "{")
	end := strings.LastIndex(content, "}")
	if start < 0 || end <= start {
		return Decision{}, fmt.Errorf("invalid llm decision format")
	}
	content = content[start : end+1]
	var d Decision
	if err = json.Unmarshal([]byte(content), &d); err != nil {
		return Decision{}, fmt.Errorf("parse decision failed: %w", err)
	}
	if len(d.Tools) == 0 {
		d.Tools = []string{"tool.file.list"}
	}
	return d, nil
}

func (p *DeepSeekProvider) Summarize(ctx context.Context, query string, decision Decision, items []any) (string, error) {
	raw, _ := json.Marshal(items)
	prompt := "你是网盘检索助手。请根据用户问题和检索结果给出简短中文总结，不要编造。" +
		"用户问题: " + query + "；路由意图: " + decision.Intent + "；结果JSON: " + string(raw)
	return p.chat(ctx, prompt)
}

func (p *DeepSeekProvider) chat(ctx context.Context, prompt string) (string, error) {
	if strings.TrimSpace(p.apiKey) == "" {
		return "", fmt.Errorf("deepseek api key is empty")
	}
	payload := map[string]any{
		"model": p.model,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
		"temperature": 0,
	}
	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+p.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("deepseek status=%d", resp.StatusCode)
	}
	var out struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err = json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	if len(out.Choices) == 0 {
		return "", fmt.Errorf("empty llm choices")
	}
	return strings.TrimSpace(out.Choices[0].Message.Content), nil
}
