package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"yyb_go/internal/pinzan"
	"yyb_go/internal/qr"
)

func newPinzanTestApp(t *testing.T) *App {
	t.Helper()
	root := t.TempDir()
	regionsFile := filepath.Join(root, "regions.txt")
	if err := os.WriteFile(regionsFile, []byte("全国 all\n广东省 440000\n深圳市 440300\n"), 0o600); err != nil {
		t.Fatalf("write regions file: %v", err)
	}
	app, err := NewApp(Config{
		ResourceRoot:      filepath.Join(root, "resource"),
		PinzanRegionsFile: regionsFile,
	})
	if err != nil {
		t.Fatalf("NewApp() error = %v", err)
	}
	t.Cleanup(func() { _ = app.Close() })
	return app
}

func TestQRProxyRequiresPinzanConfiguration(t *testing.T) {
	t.Setenv("GIN_MODE", "test")
	app := newPinzanTestApp(t)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/qr", bytes.NewBufferString(`{"use_proxy":true,"area":"440300"}`))
	request.Header.Set("Content-Type", "application/json")
	app.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusBadGateway {
		t.Fatalf("POST /qr status = %d, want %d", recorder.Code, http.StatusBadGateway)
	}
	var response apiEnvelope
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !strings.Contains(response.Msg, "YYB_PINZAN_NO") || !strings.Contains(response.Msg, "YYB_PINZAN_SECRET") {
		t.Fatalf("POST /qr message = %q", response.Msg)
	}
}

func TestPinzanRegionsEndpoint(t *testing.T) {
	t.Setenv("GIN_MODE", "test")
	app := newPinzanTestApp(t)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/pinzan/regions", nil)
	app.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("GET /pinzan/regions status = %d, want %d", recorder.Code, http.StatusOK)
	}
	var response struct {
		Code int                  `json:"code"`
		Data pinzan.RegionCatalog `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Code != 0 || response.Data.Nationwide.Code != "all" || !response.Data.HasCode("440300") {
		t.Fatalf("GET /pinzan/regions body = %#v", response)
	}
}

func TestQRProxyRejectsUnknownArea(t *testing.T) {
	t.Setenv("GIN_MODE", "test")
	app := newPinzanTestApp(t)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/qr", bytes.NewBufferString(`{"use_proxy":true,"area":"999999"}`))
	request.Header.Set("Content-Type", "application/json")
	app.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("POST /qr status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}
}

func TestExpiredProxySessionRequiresNewQRCode(t *testing.T) {
	t.Setenv("GIN_MODE", "test")
	app := newPinzanTestApp(t)
	sessionID := "expired-proxy-session"
	app.mu.Lock()
	app.qrSessions[sessionID] = &qr.Session{
		ID:        sessionID,
		CreatedAt: time.Now().Add(-time.Minute),
		ExpiresAt: time.Now().Add(-time.Second),
	}
	app.mu.Unlock()

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/qr/"+sessionID+"/poll", nil)
	app.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("GET expired poll status = %d, want %d", recorder.Code, http.StatusOK)
	}
	var response struct {
		Data qr.PollResult `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Data.Status != "expired" {
		t.Fatalf("expired poll response = %#v", response.Data)
	}
	if app.getQRSession(sessionID) != nil {
		t.Fatal("expired proxy session was not removed")
	}
}

func TestSecondsUntilRoundsPositiveDurationUp(t *testing.T) {
	if got := secondsUntil(time.Now().Add(500 * time.Millisecond)); got != 1 {
		t.Fatalf("secondsUntil() = %d, want 1", got)
	}
}
