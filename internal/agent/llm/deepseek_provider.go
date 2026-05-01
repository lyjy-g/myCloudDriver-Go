package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
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
	prompt := "你是网盘智能管家，只能从工具白名单中选择工具。白名单：" +
		"tool.file.list(文件列表), tool.file.search(文件检索带过滤), tool.file.stats(文件统计), tool.file.trash.list(回收站), tool.file.rank(重要文件排序), " +
		"tool.share.list(分享列表), tool.share.search(分享搜索), tool.share.records(访问记录), tool.share.stats(分享统计), tool.share.revoke(撤销分享), " +
		"tool.share.create(创建分享), tool.transfer.status(传输任务状态), tool.rag.search(知识库检索), tool.workflow(工作流编排)。" +
		"输出严格 JSON: {\"intent\":\"...\",\"tools\":[\"tool.file.list\"]}。" +
		"规则：文件搜索/过滤用 tool.file.search；统计计数用 tool.file.stats；回收站用 tool.file.trash.list；重要文件排序用 tool.file.rank；" +
		"分享统计用 tool.share.stats；分享记录用 tool.share.records；创建分享用 tool.share.create；撤销分享用 tool.share.revoke。用户问题: " + query
	content, err := p.chat(ctx, prompt)
	if err != nil {
		return Decision{}, err
	}
	content = sanitizeJSONBlock(content)
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
	d.Tools = normalizeTools(d.Tools)
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

func sanitizeJSONBlock(raw string) string {
	s := strings.TrimSpace(raw)
	s = strings.TrimPrefix(s, "```json")
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimSuffix(s, "```")
	return strings.TrimSpace(s)
}

func normalizeTools(tools []string) []string {
	allow := map[string]struct{}{
		"tool.file.list":       {},
		"tool.file.search":     {},
		"tool.file.stats":      {},
		"tool.file.trash.list": {},
		"tool.file.rank":       {},
		"tool.share.list":      {},
		"tool.share.search":    {},
		"tool.share.records":   {},
		"tool.share.stats":     {},
		"tool.share.revoke":    {},
		"tool.share.create":    {},
		"tool.transfer.status": {},
		"tool.rag.search":      {},
		"tool.workflow":        {},
	}
	seen := make(map[string]struct{}, len(tools))
	out := make([]string, 0, len(tools))
	for _, t := range tools {
		name := strings.TrimSpace(t)
		if _, ok := allow[name]; !ok {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		out = append(out, name)
	}
	if len(out) == 0 {
		out = []string{"tool.file.list"}
	}
	// 固定顺序，避免相同问题跨次输出工具顺序抖动。
	sort.SliceStable(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}
