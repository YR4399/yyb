package pinzan

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestNewHTTPClientUsesWhitelistAreaAndProxy(t *testing.T) {
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.String() != "http://health.test/ip" {
			t.Errorf("proxy request URL = %q", r.URL.String())
		}
		_, _ = w.Write([]byte("<html>target reachable</html>"))
	}))
	defer proxy.Close()
	proxyURL, _ := url.Parse(proxy.URL)

	var gotQuery url.Values
	extract := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query()
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code": 0,
			"data": map[string]any{"list": []map[string]any{{
				"ip": proxyURL.Hostname(), "port": proxyURL.Port(),
			}}},
		})
	}))
	defer extract.Close()

	client := NewClient(Config{
		No: "package-no", Secret: "extract-secret", Minute: 3,
		ExtractURL: extract.URL, HealthURL: "http://health.test/ip", Timeout: time.Second,
	})
	httpClient, verification, err := client.NewHTTPClient(context.Background(), "440300", time.Second)
	if err != nil {
		t.Fatalf("NewHTTPClient() error = %v", err)
	}
	httpClient.CloseIdleConnections()
	if verification.LatencyMS < 1 || time.Until(verification.ExpiresAt) <= 0 {
		t.Fatalf("verification = %#v", verification)
	}

	want := map[string]string{
		"no": "package-no", "secret": "extract-secret", "num": "1",
		"mode": "whitelist", "format": "json", "protocol": "1", "pool": "quality", "minute": "3", "area": "440300",
	}
	for key, value := range want {
		if gotQuery.Get(key) != value {
			t.Errorf("query %s = %q, want %q", key, gotQuery.Get(key), value)
		}
	}
}

func TestNewHTTPClientDefaultsMinuteOne(t *testing.T) {
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"ip": "203.0.113.9"})
	}))
	defer proxy.Close()
	proxyURL, _ := url.Parse(proxy.URL)

	var gotQuery url.Values
	extract := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query()
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code": 0,
			"data": map[string]any{"list": []map[string]any{{"ip": proxyURL.Hostname(), "port": proxyURL.Port()}}},
		})
	}))
	defer extract.Close()

	client := NewClient(Config{No: "package-no", Secret: "extract-secret", ExtractURL: extract.URL, HealthURL: "http://health.test/ip"})
	httpClient, _, err := client.NewHTTPClient(context.Background(), "", time.Second)
	if err != nil {
		t.Fatalf("NewHTTPClient() error = %v", err)
	}
	httpClient.CloseIdleConnections()
	if gotQuery.Get("area") != "all" || gotQuery.Get("pool") != "quality" {
		t.Fatalf("query area=%q pool=%q", gotQuery.Get("area"), gotQuery.Get("pool"))
	}
	if gotQuery.Get("minute") != "1" {
		t.Fatalf("minute = %q, want 1", gotQuery.Get("minute"))
	}
}

func TestNewClientChecksWechatTargetByDefault(t *testing.T) {
	client := NewClient(Config{})
	if client.cfg.HealthURL != "https://open.weixin.qq.com/" {
		t.Fatalf("health URL = %q", client.cfg.HealthURL)
	}
}

func TestNewHTTPClientReplacesFailedProxy(t *testing.T) {
	badProxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "unavailable", http.StatusBadGateway)
	}))
	defer badProxy.Close()
	goodProxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"ip": "203.0.113.10"})
	}))
	defer goodProxy.Close()
	badURL, _ := url.Parse(badProxy.URL)
	goodURL, _ := url.Parse(goodProxy.URL)

	extractCount := 0
	extract := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		proxyURL := badURL
		if extractCount > 0 {
			proxyURL = goodURL
		}
		extractCount++
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code": 0,
			"data": map[string]any{"list": []map[string]any{{"ip": proxyURL.Hostname(), "port": proxyURL.Port()}}},
		})
	}))
	defer extract.Close()

	client := NewClient(Config{No: "package-no", Secret: "extract-secret", ExtractURL: extract.URL, HealthURL: "http://health.test/ip", Timeout: time.Second})
	httpClient, verification, err := client.NewHTTPClient(context.Background(), "440100", time.Second)
	if err != nil {
		t.Fatalf("NewHTTPClient() error = %v", err)
	}
	httpClient.CloseIdleConnections()
	if extractCount != 2 {
		t.Fatalf("extract count = %d, want 2", extractCount)
	}
	if verification.LatencyMS < 1 {
		t.Fatalf("latency = %dms", verification.LatencyMS)
	}
}

func TestNewHTTPClientRejectsProviderErrorWithoutLeakingSecret(t *testing.T) {
	extract := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"code": 1001, "msg": "套餐 package-no 的密钥 do-not-leak 未通过，请先添加白名单"})
	}))
	defer extract.Close()

	client := NewClient(Config{No: "package-no", Secret: "do-not-leak", ExtractURL: extract.URL})
	_, _, err := client.NewHTTPClient(context.Background(), "广东", time.Second)
	if err == nil {
		t.Fatal("NewHTTPClient() error = nil")
	}
	if got := err.Error(); !strings.Contains(got, "请先添加白名单") || strings.Contains(got, "package-no") || strings.Contains(got, "do-not-leak") {
		t.Fatalf("error was not safely surfaced: %q", got)
	}
}

func TestNewHTTPClientSurfacesProviderErrorVariants(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{"top-level message", `{"code":1002,"message":"套餐余额不足"}`, "套餐余额不足"},
		{"nested msg", `{"code":1003,"data":{"msg":"服务器 IP 不在白名单"}}`, "服务器 IP 不在白名单"},
		{"string data", `{"code":1004,"data":"提取频率过高"}`, "提取频率过高"},
		{"code fallback", `{"code":1005,"data":null}`, "业务码 1005（响应未提供错误说明）"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			extract := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(tt.body))
			}))
			defer extract.Close()

			client := NewClient(Config{No: "package-no", Secret: "extract-secret", ExtractURL: extract.URL})
			_, _, err := client.NewHTTPClient(context.Background(), "440100", time.Second)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want substring %q", err, tt.want)
			}
		})
	}
}
