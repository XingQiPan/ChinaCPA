package fp

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"
)

// TestRealFingerprintChrome 实连 tls.peet.ws 验证伪装后的 JA3/JA4/Akamai HTTP2
// 指纹是 Chrome 形态（需要外网；离线环境跳过）。
func TestRealFingerprintChrome(t *testing.T) {
	if testing.Short() {
		t.Skip("short 模式跳过外网验证")
	}
	req, err := http.NewRequest(http.MethodGet, "https://tls.peet.ws/api/all", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := Do("test-salt", req, Opts{Timeout: 30 * time.Second})
	if err != nil {
		t.Skipf("外网不可达，跳过: %v", err)
	}
	defer resp.Body.Close()
	var out struct {
		TLS struct {
			JA3Hash string `json:"ja3_hash"`
			JA4     string `json:"ja4"`
		} `json:"tls"`
		HTTP2 struct {
			AkamaiHash string `json:"akamai_fingerprint_hash"`
		} `json:"http2"`
		HTTPVer string `json:"http_version"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	t.Logf("ja3=%s ja4=%s akamai2=%s http=%s", out.TLS.JA3Hash, out.TLS.JA4, out.HTTP2.AkamaiHash, out.HTTPVer)
	if out.TLS.JA4 == "" {
		t.Fatal("未解析到 JA4")
	}
	if !strings.HasPrefix(out.TLS.JA4, "t13") {
		t.Fatalf("JA4 形态异常: %s", out.TLS.JA4)
	}
	// Go 1.26 标准库实测基线：t13d1312h2_f57a46bbacb6_f50d94e863eb
	if out.TLS.JA4 == "t13d1312h2_f57a46bbacb6_f50d94e863eb" {
		t.Fatalf("JA4 仍是 Go 标准库指纹: %s", out.TLS.JA4)
	}
	if out.HTTPVer != "HTTP/2.0" && out.HTTPVer != "h2" {
		t.Fatalf("未走 HTTP/2: %s", out.HTTPVer)
	}
}

// TestSaltedProfilesDistinct 不同盐应命中不同 Chrome 版本池成员（指纹多样），
// 同盐必须恒定。
func TestSaltedProfilesDistinct(t *testing.T) {
	seen := map[string]string{}
	for _, salt := range []string{"qoder", "trae", "doubao", "qwen", "workbuddy", "qwenwork"} {
		id := ProfileName(salt)
		t.Logf("salt=%s -> %s", salt, id)
		if id == "" {
			t.Fatalf("salt=%s 无指纹", salt)
		}
		seen[salt] = id
	}
	if seen["qoder"] != ProfileName("qoder") {
		t.Fatal("同盐指纹不稳定")
	}
	// 6 个 provider 落到 4 个版本池：允许重复，但必须出现至少两种版本。
	kinds := map[string]bool{}
	for _, v := range seen {
		kinds[v] = true
	}
	if len(kinds) < 2 {
		t.Fatalf("指纹多样性不足: %v", seen)
	}
}
