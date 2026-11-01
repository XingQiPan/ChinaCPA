package usage

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestRecordAndAggregate 记录 → 今日聚合 → 计费估算 → 跨日重置。
func TestRecordAndAggregate(t *testing.T) {
	dir := t.TempDir()
	tr, err := New(dir, "")
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	tr.Record(Record{Provider: "qwen", Account: "u1", Model: "qwen3-coder-plus", PromptTokens: 1000000, CompletionTokens: 500000})
	tr.Record(Record{Provider: "qwen", Account: "u1", Model: "qwen3-coder-plus", PromptTokens: 1000000, CompletionTokens: 500000})
	items := tr.TodayWithCost()
	if len(items) != 1 || items[0].Requests != 2 {
		t.Fatalf("聚合错误: %+v", items)
	}
	if items[0].Total != 3000000 {
		t.Fatalf("token 合计错误: %d", items[0].Total)
	}
	// 计价：2M in * 0.002 + 1M out * 0.006 = 0.01 USD → ¥0.071
	if items[0].CostCNY < 0.070 || items[0].CostCNY > 0.072 {
		t.Fatalf("费用估算错误: %f", items[0].CostCNY)
	}
	// 请求数查询（qwen 额度回填）
	if n := tr.RequestsToday("qwen", "u1"); n != 2 {
		t.Fatalf("RequestsToday 错误: %d", n)
	}
}

// TestRestoreToday 重启回填：新建 Tracker 从日志恢复今日计数。
func TestRestoreToday(t *testing.T) {
	dir := t.TempDir()
	tr, _ := New(dir, "")
	tr.Record(Record{Provider: "trae", Account: "x", Model: "glm-5.2", PromptTokens: 10, CompletionTokens: 20})
	// 模拟重启
	tr2, err := New(dir, "")
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	if n := tr2.RequestsToday("trae", "x"); n != 1 {
		t.Fatalf("重启回填失败: %d", n)
	}
	items := tr2.TodayWithCost()
	if len(items) != 1 || items[0].Total != 30 {
		t.Fatalf("重启聚合失败: %+v", items)
	}
}

// TestHistoryDays 近 N 日。
func TestHistoryDays(t *testing.T) {
	dir := t.TempDir()
	tr, _ := New(dir, "")
	tr.Record(Record{Provider: "qwen", Account: "u", Model: "m", PromptTokens: 5, CompletionTokens: 5})
	h := tr.HistoryDays(7)
	if len(h) != 7 {
		t.Fatalf("天数错误: %d", len(h))
	}
	today := time.Now().Format("2006-01-02")
	var found bool
	for _, d := range h {
		if d.Day == today && d.Total == 10 {
			found = true
		}
	}
	if !found {
		t.Fatalf("今日数据缺失: %+v", h)
	}
}

// TestPricingFile 自定义单价表加载。
func TestPricingFile(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "p.json")
	os.WriteFile(f, []byte(`{"usd_cny":7.0,"models":{"my-model":{"input":1,"output":2}}}`), 0o644)
	tr, _ := New(t.TempDir(), f)
	c := tr.Cost("my-model", 1000000, 1000000)
	if c < 20.99 || c > 21.01 { // 1*1 + 1*2 = 3 USD * 7 = 21
		t.Fatalf("自定义计价错误: %f", c)
	}
}

// TestCallsPerUnit 按次计费（图片/视频）：Calls/Unit 记账、聚合、重启回放与计价。
func TestCallsPerUnit(t *testing.T) {
	dir := t.TempDir()
	tr, err := New(dir, "")
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	// 发布模型按次单价 $0.02/张
	tr.SetPublishedPricing(map[string]PubPrice{
		"seedream": {PerCall: 0.02},
	}, 7.0)
	tr.Record(Record{Provider: "doubao", Account: "u1", Model: "seedream", Calls: 3, Unit: "images"})
	items := tr.TodayWithCost()
	if len(items) != 1 || items[0].Calls != 3 {
		t.Fatalf("聚合错误: %+v", items)
	}
	// 3 张 * $0.02 * 7 = ¥0.42
	if items[0].CostCNY < 0.41 || items[0].CostCNY > 0.43 {
		t.Fatalf("按次计价错误: %f", items[0].CostCNY)
	}
	if c := tr.CostCalls("seedream", 3); c < 0.41 || c > 0.43 {
		t.Fatalf("CostCalls 错误: %f", c)
	}
	// 重启回放保留 Calls/Unit
	tr2, err := New(dir, "")
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	items2 := tr2.TodayWithCost()
	if len(items2) != 1 || items2[0].Calls != 3 || items2[0].Unit != "images" {
		t.Fatalf("重启回放失败: %+v", items2)
	}
	// HistoryDays 带 Calls
	h := tr2.HistoryDays(1)
	if h[0].Calls != 3 {
		t.Fatalf("HistoryDays 丢失 Calls: %+v", h)
	}
}

// TestModelsRange 按模型区间聚合：今日走内存聚合，历史日扫描落盘日志，含指定日期与空区间。
func TestModelsRange(t *testing.T) {
	dir := t.TempDir()
	tr, err := New(dir, "")
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	now := time.Now()
	today := now.Format("2006-01-02")
	yesterday := now.AddDate(0, 0, -1).Format("2006-01-02")
	// 今日：两个模型
	tr.Record(Record{Provider: "qwen", Account: "u1", Model: "qwen3-coder-plus", PromptTokens: 100, CompletionTokens: 50})
	tr.Record(Record{Provider: "qwen", Account: "u2", Model: "glm-5.2", PromptTokens: 30, CompletionTokens: 20})
	// 昨天：直接写日志文件（跨日不走 Record，避免重置今日内存聚合后写错文件）
	day := now.AddDate(0, 0, -1)
	rec := Record{TS: time.Date(day.Year(), day.Month(), day.Day(), 12, 0, 0, 0, day.Location()).Unix(), Provider: "trae", Account: "x", Model: "glm-5.2", PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15}
	raw, _ := json.Marshal(rec)
	fn := filepath.Join(dir, "usage-"+strings.ReplaceAll(yesterday, "-", "")+".jsonl")
	if err := os.WriteFile(fn, append(raw, '\n'), 0o644); err != nil {
		t.Fatalf("写日志: %v", err)
	}

	// 今日：模型合计（跨账号合并）
	m := tr.ModelsRange(today, today)
	if len(m) != 2 {
		t.Fatalf("今日模型数错误: %+v", m)
	}
	if m[0].Model != "qwen3-coder-plus" || m[0].Total != 150 { // 按 Total 降序
		t.Fatalf("今日排序/合计错误: %+v", m)
	}
	// 昨天：扫描文件
	my := tr.ModelsRange(yesterday, yesterday)
	if len(my) != 1 || my[0].Model != "glm-5.2" || my[0].Requests != 1 || my[0].Total != 15 {
		t.Fatalf("昨日聚合错误: %+v", my)
	}
	// 近 7 天：今日 + 昨日合并（glm-5.2 跨账号合并 = 30+20+10+5=65）
	m7 := tr.ModelsRange(now.AddDate(0, 0, -6).Format("2006-01-02"), today)
	if len(m7) != 2 {
		t.Fatalf("7 天模型数错误: %+v", m7)
	}
	var glm *ModelUsage
	for i := range m7 {
		if m7[i].Model == "glm-5.2" {
			glm = &m7[i]
		}
	}
	if glm == nil || glm.Total != 65 || glm.Requests != 2 {
		t.Fatalf("7 天跨账号合并错误: %+v", m7)
	}
	// 汇总
	sum := SumModels(m7)
	if sum.Total != 150+65 || sum.Requests != 3 {
		t.Fatalf("SumModels 错误: %+v", sum)
	}
	// 无数据的日期 → 空列表
	if m0 := tr.ModelsRange(now.AddDate(0, 0, -3).Format("2006-01-02"), now.AddDate(0, 0, -3).Format("2006-01-02")); len(m0) != 0 {
		t.Fatalf("空日期应返回空列表: %+v", m0)
	}
	// 非法参数
	if tr.ModelsRange("bad", today) != nil || tr.ModelsRange(today, "bad") != nil {
		t.Fatalf("非法日期应返回 nil")
	}
}
