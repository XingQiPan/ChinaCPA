package reqlog

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAggregateChannels(t *testing.T) {
	dir := t.TempDir()
	db, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		db.Close()
		os.RemoveAll(dir)
	}()

	// 同模型、两渠道、多请求
	db.Add(Entry{Provider: "qwenwork", Model: "qwen-3.8-flash", OK: true, PromptTokens: 100, CompletionTokens: 50})
	db.Add(Entry{Provider: "qwenwork", Model: "qwen-3.8-flash", OK: true, PromptTokens: 300, CompletionTokens: 150, CacheTokens: 200})
	db.Add(Entry{Provider: "trae", Model: "qwen-3.8-flash", OK: true, PromptTokens: 500, CompletionTokens: 250})
	db.Add(Entry{Provider: "trae", Model: "qwen-3.8-flash", OK: false, PromptTokens: 999, CompletionTokens: 999}) // 失败不计
	db.Add(Entry{Provider: "qwenwork", Model: "other", OK: true, PromptTokens: 10, CompletionTokens: 5})

	aggs := db.AggregateChannels("qwen-3.8-flash", 0)
	if len(aggs) != 2 {
		t.Fatalf("期望 2 个渠道，得到 %d: %+v", len(aggs), aggs)
	}
	idx := map[string]ChannelAgg{}
	for _, a := range aggs {
		idx[a.Provider] = a
	}
	qw := idx["qwenwork"]
	if qw.Requests != 2 {
		t.Errorf("qwenwork 请求数=%d 期望 2", qw.Requests)
	}
	if qw.AvgPrompt != 200 {
		t.Errorf("qwenwork 平均输入=%v 期望 200", qw.AvgPrompt)
	}
	if qw.AvgCompletion != 100 {
		t.Errorf("qwenwork 平均输出=%v 期望 100", qw.AvgCompletion)
	}
	if qw.AvgCache != 100 {
		t.Errorf("qwenwork 平均缓存=%v 期望 100", qw.AvgCache)
	}
	tr := idx["trae"]
	if tr.Requests != 1 {
		t.Errorf("trae 请求数=%d 期望 1（失败请求被排除）", tr.Requests)
	}
	if tr.AvgPrompt != 500 {
		t.Errorf("trae 平均输入=%v 期望 500", tr.AvgPrompt)
	}
}

func TestOpenCreatesFile(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "usage")
	db, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := os.Stat(filepath.Join(dir, "requests.db")); err != nil {
		t.Fatalf("requests.db 未创建: %v", err)
	}
}