package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// TestLoadMissingFile 配置文件不存在时应返回可用的默认配置。
// 回归：Default() 未经 normalize 时 TLSFingerprint 为 nil，调用方解引用会
// panic（首次运行直接崩溃）。
func TestLoadMissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load 应返回默认配置而非错误: %v", err)
	}
	if cfg == nil {
		t.Fatal("配置为 nil")
	}
	if cfg.TLSFingerprint == nil {
		t.Fatal("TLSFingerprint 仍为 nil（normalize 未生效）")
	}
	if !*cfg.TLSFingerprint {
		t.Error("默认应启用 TLS 指纹")
	}
	if cfg.Listen == "" || cfg.DataDir == "" || cfg.APIKey == "" {
		t.Errorf("默认配置字段缺失: %+v", cfg)
	}
	if cfg.Path != path {
		t.Errorf("Path=%q 期望 %q", cfg.Path, path)
	}
}

// TestSaveExampleComplete 示例配置应写出 normalize 后的完整字段。
func TestSaveExampleComplete(t *testing.T) {
	path := filepath.Join(t.TempDir(), "example.json")
	if err := SaveExample(path); err != nil {
		t.Fatalf("SaveExample: %v", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("生成的配置不是合法 JSON: %v", err)
	}
	if _, ok := m["tls_fingerprint"]; !ok {
		t.Error("示例配置缺少 tls_fingerprint 字段")
	}
	// 生成的配置应能被 Load 正常读回
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("回读失败: %v", err)
	}
	if cfg.TLSFingerprint == nil {
		t.Error("回读后 TLSFingerprint 为 nil")
	}
}
