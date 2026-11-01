package gateway

import (
	"encoding/json"
	"strings"
	"testing"

	"chinacpa/internal/provider"
)

// TestAggregateToolCalls 工具调用分片按 index 合并。
func TestAggregateToolCalls(t *testing.T) {
	sse := "" +
		"data: {\"choices\":[{\"delta\":{\"tool_calls\":[{\"index\":0,\"id\":\"call_1\",\"type\":\"function\",\"function\":{\"name\":\"get\",\"arguments\":\"{\\\"a\\\"\"}}]}}]}\n\n" +
		"data: {\"choices\":[{\"delta\":{\"tool_calls\":[{\"index\":0,\"function\":{\"arguments\":\":1}\"}}]}}]}\n\n" +
		"data: {\"choices\":[{\"delta\":{\"tool_calls\":[{\"index\":1,\"id\":\"call_2\",\"type\":\"function\",\"function\":{\"name\":\"set\",\"arguments\":\"{\\\"b\\\":2}\"}}]}}]}\n\n" +
		"data: [DONE]\n\n"
	out, _, _, err := provider.AggregateOpenAISSE(strings.NewReader(sse), "pub-m")
	if err != nil {
		t.Fatalf("aggregate: %v", err)
	}
	var resp struct {
		Choices []struct {
			Message struct {
				ToolCalls []struct {
					ID       string `json:"id"`
					Function struct {
						Name      string `json:"name"`
						Arguments string `json:"arguments"`
					} `json:"function"`
				} `json:"tool_calls"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(out, &resp); err != nil {
		t.Fatalf("unmarshal: %v (out=%s)", err, out)
	}
	tc := resp.Choices[0].Message.ToolCalls
	if len(tc) != 2 {
		t.Fatalf("期望 2 个合并后的工具调用，实际 %d: %+v", len(tc), tc)
	}
	if tc[0].ID != "call_1" || tc[0].Function.Name != "get" || tc[0].Function.Arguments != "{\"a\":1}" {
		t.Fatalf("工具调用 0 合并错误: %+v", tc[0])
	}
	if tc[1].ID != "call_2" || tc[1].Function.Arguments != "{\"b\":2}" {
		t.Fatalf("工具调用 1 合并错误: %+v", tc[1])
	}
	if resp.Choices[0].FinishReason != "tool_calls" {
		t.Fatalf("有工具调用且无 content 时应 finish=tool_calls，实际 %q", resp.Choices[0].FinishReason)
	}
}

// TestAggregateContent 普通文本聚合 + usage 保留。
func TestAggregateContent(t *testing.T) {
	sse := "" +
		"data: {\"choices\":[{\"delta\":{\"content\":\"你\"}}]}\n\n" +
		"data: {\"choices\":[{\"delta\":{\"content\":\"好\"}}],\"usage\":{\"prompt_tokens\":10,\"completion_tokens\":2,\"total_tokens\":12}}\n\n" +
		"data: {\"choices\":[{\"delta\":{},\"finish_reason\":\"stop\"}]}\n\n" +
		"data: [DONE]\n\n"
	out, _, finish, err := provider.AggregateOpenAISSE(strings.NewReader(sse), "m")
	if err != nil {
		t.Fatalf("aggregate: %v", err)
	}
	var resp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int64 `json:"prompt_tokens"`
			CompletionTokens int64 `json:"completion_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(out, &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Choices[0].Message.Content != "你好" {
		t.Fatalf("内容聚合错误: %q", resp.Choices[0].Message.Content)
	}
	if resp.Usage.PromptTokens != 10 || resp.Usage.CompletionTokens != 2 {
		t.Fatalf("usage 保留错误: %+v", resp.Usage)
	}
	if finish != "stop" {
		t.Fatalf("finish 错误: %q", finish)
	}
}

// TestRetryableChannelErr 渠道内重试判定：限流/上游错误可重试，
// 配额/凭证/客户端错误不重试；本轮没试到任何账号（trail 空）也不重试。
func TestRetryableChannelErr(t *testing.T) {
	cases := []struct {
		name  string
		err   *provider.Error
		trail string
		want  bool
	}{
		{"429 限流", &provider.Error{Kind: provider.ErrRate, Status: 429}, "a1 ✗429", true},
		{"软限流", &provider.Error{Kind: provider.ErrRate}, "a1 ✗请求过于频繁", true},
		{"上游故障", &provider.Error{Kind: provider.ErrUpstream}, "a1 ✗超时", true},
		{"配额耗尽", &provider.Error{Kind: provider.ErrQuota}, "a1 ✗Billing", false},
		{"凭证失效", &provider.Error{Kind: provider.ErrAuth}, "a1 ✗401", false},
		{"客户端错误", &provider.Error{Kind: provider.ErrClient}, "a1 ✗400", false},
		{"无账号可选", &provider.Error{Kind: provider.ErrUpstream}, "", false},
		{"无错误", nil, "a1 ✗HTTP 500", false},
	}
	for _, c := range cases {
		if got := retryableChannelErr(c.err, c.trail); got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}

// TestRetryableMediaErr 媒体渠道重试判定：裸 error 归一分类后同 chat 语义。
func TestRetryableMediaErr(t *testing.T) {
	if !retryableMediaErr(&provider.Error{Kind: provider.ErrRate, Status: 429}, true) {
		t.Error("429 且有账号尝试应可重试")
	}
	if retryableMediaErr(&provider.Error{Kind: provider.ErrQuota}, true) {
		t.Error("配额耗尽不应重试")
	}
	if retryableMediaErr(&provider.Error{Kind: provider.ErrRate}, false) {
		t.Error("未试到任何账号不应重试")
	}
}
