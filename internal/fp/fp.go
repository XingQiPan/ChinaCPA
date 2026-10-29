// Package fp 为所有上游请求提供浏览器 TLS 指纹伪装（JA3 / JA4 / Akamai HTTP2）。
//
// 背景：本项目此前全部使用 net/http 默认 Transport，TLS ClientHello 是
// 标准 Go 指纹——上游风控只要算一下握手哈希，就能识破「UA 写着 Chrome、
// 客户端却是 Go 程序」的矛盾。本包用 bogdanfinn/tls-client（fhttp + utls
// 栈）重建出站客户端：ClientHello 扩展顺序、HTTP/2 设置帧、窗口增量、
// 伪头顺序全部对齐真实 Chrome（实测 tls.peet.ws：t13d1516h2_8daaf6152771
// 形态，HTTP/2 协商正常）。
//
// 按账号稳定化：调用方为每个「设备」传一个稳定的盐（provider:账号设备标识）。
// 同一盐恒定复用同一个伪装客户端（同一连接池 + 同一 TLS 会话缓存 + 同一
// Chrome 版本），对上游表现为「同一台设备持续在线」；不同账号盐不同、指纹
// 不同，避免所有账号共用一台"虚拟设备"被风控聚类。GREASE 值保持每连接
// 随机（与真实 Chrome 一致）。
//
// 调用方继续使用标准库 net/http 的 Request/Response 类型，Do 内部做
// fhttp 镜像转换；context 取消、超时、请求体透传语义与 http.Client 对齐。
//
// 禁用方式：config.json 置 "tls_fingerprint": false（SetEnabled），
// 全部调用点退回共享标准 Transport，行为与旧版一致。
package fp

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"io"
	"log"
	"net/http"
	"sync"
	"time"

	fhttp "github.com/bogdanfinn/fhttp"
	tls_client "github.com/bogdanfinn/tls-client"
	"github.com/bogdanfinn/tls-client/profiles"
)

// enabled 全局开关，由 main 在加载配置后调用 SetEnabled 设置。
var enabled = true

// SetEnabled 开关 TLS 指纹伪装；false 时 Do 走标准 net/http。
func SetEnabled(on bool) { enabled = on }

// Enabled 报告当前是否启用指纹伪装。
func Enabled() bool { return enabled }

// Opts 单次请求的传输选项（语义对齐旧 http.Client 字段）。
type Opts struct {
	// Timeout 整体超时（含读响应体，与 http.Client.Timeout 一致；0 表示不限，
	// 用于流式/SSE）。
	Timeout time.Duration
	// NoRedirect 不自动跟随重定向（对齐 CheckRedirect=ErrUseLastResponse，
	// 豆包登录链路手动收每跳 Set-Cookie）。
	NoRedirect bool
}

// salted 返回缓存键：盐 + 非默认选项（同一 provider 选项固定，正常只占一个键）。
func cacheKey(salt string, o Opts) string {
	k := salt
	if o.NoRedirect {
		k += "|noredir"
	}
	return k
}

// chromeProfiles 指纹版本池（按真实 Chrome 新旧排序，盐哈希决定每台"设备"
// 长期固定使用哪一版）。
var chromeProfiles = []profiles.ClientProfile{
	profiles.Chrome_150,
	profiles.Chrome_133,
	profiles.Chrome_124,
	profiles.Chrome_120,
}

var (
	mu      sync.Mutex
	clients = map[string]tls_client.HttpClient{} // 键 -> 伪装客户端（含连接池）
	initErr bool
)

// clientFor 返回盐对应的伪装 HttpClient（缓存复用）。失败返回 nil 并由调用方降级。
func clientFor(salt string, o Opts) tls_client.HttpClient {
	key := cacheKey(salt, o)
	mu.Lock()
	defer mu.Unlock()
	if initErr {
		return nil
	}
	if c, ok := clients[key]; ok {
		return c
	}
	h := sha256.Sum256([]byte("chinacpa-fp:" + salt))
	prof := chromeProfiles[binary.LittleEndian.Uint32(h[:4])%uint32(len(chromeProfiles))]
	opts := []tls_client.HttpClientOption{
		tls_client.WithClientProfile(prof),
		tls_client.WithTimeoutSeconds(0), // 超时由 ctx 控制（对齐 http.Client.Timeout 语义）
	}
	if o.NoRedirect {
		opts = append(opts, tls_client.WithNotFollowRedirects())
	}
	c, err := tls_client.NewHttpClient(tls_client.NewNoopLogger(), opts...)
	if err != nil { // 理论不可达（静态 profile）；本进程永久降级标准栈。
		log.Printf("fp: 伪装客户端初始化失败（%v），TLS 指纹伪装已禁用", err)
		initErr = true
		enabled = false
		return nil
	}
	clients[key] = c
	return c
}

// ProfileName 返回盐选定的 Chrome 指纹版本名（如 "Chrome-133"），供展示/测试。
func ProfileName(salt string) string {
	h := sha256.Sum256([]byte("chinacpa-fp:" + salt))
	return chromeProfiles[binary.LittleEndian.Uint32(h[:4])%uint32(len(chromeProfiles))].GetClientHelloStr()
}

// Do 以指定设备盐的浏览器指纹执行请求并原样返回响应（body 由调用方关闭）。
// 伪装关闭（或初始化失败自动降级）时走共享标准 Transport，行为同旧版。
func Do(salt string, req *http.Request, o Opts) (*http.Response, error) {
	if !Enabled() {
		return standardClient(o).Do(req)
	}
	c := clientFor(salt, o)
	if c == nil {
		return standardClient(o).Do(req)
	}
	ctx := req.Context()
	var cancel context.CancelFunc
	if o.Timeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, o.Timeout)
		defer func() {
			// 仅当失败路径未移交所有权时取消；移交后 cancel 已为 nil。
			if cancel != nil {
				cancel()
			}
		}()
	}
	fr, err := mirrorRequest(req.WithContext(ctx))
	if err != nil {
		return nil, err
	}
	resp, err := c.Do(fr)
	if err != nil {
		return nil, err
	}
	out := mirrorResponse(resp)
	if cancel != nil {
		// 把取消时机移交到 Body 关闭，复刻 http.Client.Timeout 语义。
		out.Body = &cancelOnClose{rc: resp.Body, cancel: cancel}
		cancel = nil
	}
	return out, nil
}

// cancelOnClose 把 context 取消的生命周期挂到 Body 关闭上，
// 复刻 http.Client.Timeout「计时持续到 Body 关闭」的语义。
type cancelOnClose struct {
	rc     io.ReadCloser
	cancel context.CancelFunc
	once   sync.Once
}

func (c *cancelOnClose) Read(p []byte) (int, error) { return c.rc.Read(p) }
func (c *cancelOnClose) Close() error {
	err := c.rc.Close()
	c.once.Do(c.cancel)
	return err
}

// mirrorRequest 把标准库请求镜像为 fhttp.Request。
func mirrorRequest(req *http.Request) (*fhttp.Request, error) {
	fr, err := fhttp.NewRequestWithContext(req.Context(), req.Method, req.URL.String(), req.Body)
	if err != nil {
		return nil, err
	}
	fr.Header = fhttp.Header{}
	for k, vs := range req.Header {
		fr.Header[k] = append([]string(nil), vs...)
	}
	fr.ContentLength = req.ContentLength
	fr.Host = req.Host
	fr.Form = req.Form
	return fr, nil
}

// mirrorResponse 把 fhttp 响应镜像回标准库响应；body 直接透传（同一个流）。
func mirrorResponse(resp *fhttp.Response) *http.Response {
	out := &http.Response{
		Status:           resp.Status,
		StatusCode:       resp.StatusCode,
		Proto:            resp.Proto,
		ProtoMajor:       resp.ProtoMajor,
		ProtoMinor:       resp.ProtoMinor,
		Header:           http.Header{},
		Body:             resp.Body,
		ContentLength:    resp.ContentLength,
		TransferEncoding: resp.TransferEncoding,
		Close:            resp.Close,
		Uncompressed:     resp.Uncompressed,
		Trailer:          http.Header{},
	}
	for k, vs := range resp.Header {
		out.Header[k] = append([]string(nil), vs...)
	}
	for k, vs := range resp.Trailer {
		out.Trailer[k] = append([]string(nil), vs...)
	}
	if resp.Request != nil && resp.Request.URL != nil {
		out.Request = &http.Request{URL: resp.Request.URL, Method: resp.Request.Method}
	}
	return out
}

// ── 标准库降级路径（开关关闭或初始化失败时使用） ─────────

var (
	stdOnce    sync.Once
	stdFollow  *http.Client
	stdNoRedir *http.Client
)

func standardClient(o Opts) *http.Client {
	stdOnce.Do(func() {
		tr := &http.Transport{
			MaxIdleConns:          200,
			MaxIdleConnsPerHost:   32,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
			ForceAttemptHTTP2:     true,
		}
		stdFollow = &http.Client{Transport: tr}
		stdNoRedir = &http.Client{
			Transport:     tr,
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		}
	})
	cl := stdFollow
	if o.NoRedirect {
		cl = stdNoRedir
	}
	return cl
}
