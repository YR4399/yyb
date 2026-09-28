package httpapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	httpSwagger "github.com/swaggo/http-swagger/v2"

	assets "yyb_go"
	"yyb_go/internal/pinzan"
	"yyb_go/internal/protocol"
	"yyb_go/internal/qr"
	"yyb_go/internal/store"
)

type Config struct {
	ResourceRoot      string
	DBFilename        string
	TCPProxy          string
	SessionTTL        time.Duration
	RequestTimeout    time.Duration
	AvatarTimeout     time.Duration
	ScanTimeout       time.Duration
	QRSessionTTL      time.Duration
	PinzanNo          string
	PinzanSecret      string
	PinzanMinute      int
	PinzanRegionsFile string
	APIToken          string // 可选：设置后 /api/auth/validate 校验此令牌
	ProxyTestTimeout  time.Duration
}

type App struct {
	cfg           Config
	resources     resources
	db            *store.DB
	pool          *protocol.Pool
	qr            *qr.Client
	pinzan        *pinzan.Client
	pinzanRegions *pinzan.RegionCatalog

	mu         sync.Mutex
	qrSessions map[string]*qr.Session
}

var swaggerDocsHandler = httpSwagger.Handler(
	httpSwagger.URL("/openapi.json"),
	httpSwagger.DocExpansion("list"),
	httpSwagger.DeepLinking(true),
	httpSwagger.DefaultModelsExpandDepth(httpSwagger.ShowModel),
)

func NewApp(cfg Config) (*App, error) {
	if cfg.ResourceRoot == "" {
		cfg.ResourceRoot = filepath.Join(".", "resource")
	}
	if cfg.DBFilename == "" {
		cfg.DBFilename = DefaultDBFilename
	}
	if cfg.RequestTimeout == 0 {
		cfg.RequestTimeout = 8 * time.Second
	}
	if cfg.AvatarTimeout == 0 {
		cfg.AvatarTimeout = 10 * time.Second
	}
	if cfg.SessionTTL == 0 {
		cfg.SessionTTL = 30 * time.Minute
	}
	if cfg.QRSessionTTL == 0 {
		cfg.QRSessionTTL = 5 * time.Minute
	}
	res, err := ensureResources(cfg.ResourceRoot)
	if err != nil {
		return nil, err
	}
	dbPath, err := prepareDBPath(res.DB, cfg.DBFilename)
	if err != nil {
		return nil, err
	}
	db, err := store.Open(dbPath)
	if err != nil {
		return nil, err
	}
	poolCfg := protocol.DefaultConfig()
	poolCfg.SessionTTL = cfg.SessionTTL
	poolCfg.ShortlinkTimeout = cfg.RequestTimeout
	poolCfg.TCPProxy = cfg.TCPProxy
	pool := protocol.NewPool(poolCfg, db)
	regionData := assets.PinzanRegions
	if cfg.PinzanRegionsFile != "" {
		data, err := os.ReadFile(cfg.PinzanRegionsFile)
		if err == nil {
			regionData = data
		} else if !os.IsNotExist(err) || filepath.Clean(cfg.PinzanRegionsFile) != filepath.Clean("地区表.txt") {
			_ = db.Close()
			return nil, errors.New("读取品赞地区表失败: " + err.Error())
		}
	}
	parsedRegions, err := pinzan.ParseRegionCatalog(regionData)
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	proxyClient := pinzan.NewClient(pinzan.Config{
		No:      cfg.PinzanNo,
		Secret:  cfg.PinzanSecret,
		Minute:  cfg.PinzanMinute,
		Timeout: cfg.RequestTimeout,
	})
	return &App{
		cfg:           cfg,
		resources:     res,
		db:            db,
		pool:          pool,
		qr:            qr.NewClient(cfg.RequestTimeout),
		pinzan:        proxyClient,
		pinzanRegions: &parsedRegions,
		qrSessions:    map[string]*qr.Session{},
	}, nil
}

func (a *App) Close() error {
	if a.db != nil {
		return a.db.Close()
	}
	return nil
}

func (a *App) Handler() http.Handler {
	if os.Getenv(gin.EnvGinMode) == "" {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()
	router.Use(gin.Logger(), gin.Recovery())

	router.Any("/", gin.WrapF(a.handleIndex))
	router.Any("/scan", gin.WrapF(a.handleScan))
	router.Any("/docs", func(c *gin.Context) {
		c.Redirect(http.StatusMovedPermanently, "/docs/index.html")
	})
	router.Any("/docs/*path", gin.WrapF(a.handleDocs))
	router.Any("/openapi.json", gin.WrapF(a.handleOpenAPI))
	router.Any("/pinzan/regions", gin.WrapF(a.handlePinzanRegions))
	router.Any("/health", func(c *gin.Context) {
		writeJSON(c.Writer, http.StatusOK, gin.H{"ok": true})
	})
	router.StaticFS("/static", http.Dir(a.resources.Static))
	router.Any("/qr", gin.WrapF(a.handleQRRoot))
	router.Any("/qr/*path", gin.WrapF(a.handleQR))
	router.Any("/accounts", gin.WrapF(a.handleAccountsRoot))
	router.Any("/accounts/avatar", gin.WrapF(a.handleAccountAvatar))
	router.Any("/accounts/refresh", gin.WrapF(a.handleAccountRefresh))
	router.Any("/accounts/resync", gin.WrapF(a.handleAccountResync))
	router.Any("/wxapp/getCode", gin.WrapF(a.handleGetCode))
	router.Any("/wxapp/getPhoneNumber", gin.WrapF(a.handleGetPhoneNumber))
	router.Any("/wxapp/operateWxData", gin.WrapF(a.handleOperateWXData))

	// ---- WCS 兼容接口层（仿照 WCS 的 /api/* 与 /wx/* 路径）----
	router.Any("/api/health", func(c *gin.Context) {
		writeJSON(c.Writer, http.StatusOK, gin.H{"ok": true})
	})
	router.Any("/api/accounts", gin.WrapF(a.handleAPIAccounts))
	router.Any("/api/accounts/add", gin.WrapF(a.handleAccountAdd))
	router.Any("/api/accounts/delete", gin.WrapF(a.handleAccountDeleteAPI))
	router.Any("/api/accounts/disable", gin.WrapF(a.handleAccountDisable))
	router.Any("/api/accounts/remark", gin.WrapF(a.handleAccountRemark))
	router.Any("/api/accounts/rescan", gin.WrapF(a.handleAccountRescanAPI))
	router.Any("/api/accounts/status", gin.WrapF(a.handleAccountStatus))
	router.Any("/api/qr/start", gin.WrapF(a.handleAPIQRStart))
	router.Any("/api/qr/status", gin.WrapF(a.handleAPIQRStatus))
	router.Any("/api/auth/validate", gin.WrapF(a.handleAuthValidate))
	router.Any("/api/path", func(c *gin.Context) {
		writeJSON(c.Writer, http.StatusOK, gin.H{"path": "/"})
	})
	router.Any("/api/proxies", gin.WrapF(a.handleProxiesList))
	router.Any("/api/proxies/add", gin.WrapF(a.handleProxyAdd))
	router.Any("/api/proxies/delete", gin.WrapF(a.handleProxyDelete))
	router.Any("/api/proxies/test", gin.WrapF(a.handleProxyTest))
	// WCS /wx/* 业务接口（与 /wxapp/* 对齐，并补齐 store 支持的读接口）
	router.Any("/wx/code", gin.WrapF(a.handleWXCode))
	router.Any("/wx/getphonenumber", gin.WrapF(a.handleWXGetPhoneNumber))
	router.Any("/wx/operateWxData", gin.WrapF(a.handleWXOperate))
	router.Any("/wx/getuserinfo", gin.WrapF(a.handleWXGetUserInfo))
	router.Any("/wx/getsession", gin.WrapF(a.handleWXGetSession))
	router.Any("/wx/refresh", gin.WrapF(a.handleWXRefresh))
	// WCS 的其它 /wx/* 操作：yyb 核心协议层未实现，逐条显式注册并返回清晰说明
	// （Gin 不允许 /wx/*path 通配与上面的静态路由共存）。
	router.Any("/wx/autoauth", gin.WrapF(a.handleWXUnsupported))
	router.Any("/wx/cloud", gin.WrapF(a.handleWXUnsupported))
	router.Any("/wx/downloadurl", gin.WrapF(a.handleWXUnsupported))
	router.Any("/wx/encryptkey", gin.WrapF(a.handleWXUnsupported))
	router.Any("/wx/gateway", gin.WrapF(a.handleWXUnsupported))
	router.Any("/wx/gateway/call", gin.WrapF(a.handleWXUnsupported))
	router.Any("/wx/oauth", gin.WrapF(a.handleWXUnsupported))
	router.Any("/wx/qrcodeauth", gin.WrapF(a.handleWXUnsupported))
	router.Any("/wx/translatelink", gin.WrapF(a.handleWXUnsupported))
	router.Any("/wx/appmsgext", gin.WrapF(a.handleWXUnsupported))
	router.Any("/wx/appmsglike", gin.WrapF(a.handleWXUnsupported))
	router.NoRoute(func(c *gin.Context) {
		writeError(c.Writer, http.StatusNotFound, "not found")
	})

	return router
}

func (a *App) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	serveHTML(w, assets.IndexHTML)
}

func (a *App) handleScan(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	serveHTML(w, assets.ScanHTML)
}

func (a *App) handleDocs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if r.URL.Path == "/docs/" {
		http.Redirect(w, r, "/docs/index.html", http.StatusMovedPermanently)
		return
	}
	swaggerDocsHandler.ServeHTTP(w, r)
}

func (a *App) handleOpenAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	writeRawJSON(w, http.StatusOK, openAPISpec)
}

func (a *App) handlePinzanRegions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if a.pinzanRegions == nil {
		writeError(w, http.StatusServiceUnavailable, "品赞地区表未配置")
		return
	}
	writeJSON(w, http.StatusOK, a.pinzanRegions)
}

func (a *App) handleQRRoot(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/qr" {
		writeError(w, http.StatusNotFound, "qr session not found")
		return
	}
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var body qrCreateRequest
	if err := decodeOptionalJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	wantBase64 := r.URL.Query().Get("as_base64") == "true"
	out, status, err := a.createQR(r.Context(), body.UseProxy, body.Area, wantBase64)
	if err != nil {
		writeError(w, status, err.Error())
		return
	}
	writeJSON(w, status, out)
}

// createQR 生成一个新的扫码登录会话，供 /qr 与 WCS 兼容的 /api/qr/start 共用。
// 返回 (响应体, HTTP状态码, error)。error 非 nil 时状态码为对应错误码。
func (a *App) createQR(ctx context.Context, useProxy bool, area string, wantBase64 bool) (map[string]any, int, error) {
	area = strings.TrimSpace(area)
	a.pruneQR()
	ctx, cancel := context.WithTimeout(ctx, 5*a.cfg.RequestTimeout+5*time.Second)
	defer cancel()
	var qrHTTPClient *http.Client
	var proxyVerification pinzan.Verification
	areaCode := ""
	if useProxy {
		areaCode = area
		if areaCode == "" {
			areaCode = "all"
		}
		if !pinzan.ValidRegionCode(areaCode) {
			return nil, http.StatusBadRequest, errors.New("area must be all or a six-digit Pinzan region code")
		}
		if a.pinzanRegions == nil || !a.pinzanRegions.HasCode(areaCode) {
			return nil, http.StatusBadRequest, errors.New("area is not present in the Pinzan region table")
		}
		var err error
		qrHTTPClient, proxyVerification, err = a.pinzan.NewHTTPClient(ctx, areaCode, a.cfg.RequestTimeout)
		if err != nil {
			return nil, http.StatusBadGateway, err
		}
	}
	img, err := a.qr.GetQRCodeImage(ctx, qrHTTPClient)
	if err != nil {
		if qrHTTPClient != nil {
			qrHTTPClient.CloseIdleConnections()
		}
		return nil, http.StatusBadGateway, err
	}
	expiresIn := 110
	var proxyLatencyMS any
	if useProxy {
		img.Session.ExpiresAt = proxyVerification.ExpiresAt
		expiresIn = secondsUntil(img.Session.ExpiresAt)
		if expiresIn == 0 {
			img.Session.Close()
			return nil, http.StatusBadGateway, errors.New("品赞代理在二维码生成前已过期，请重新获取")
		}
		proxyLatencyMS = proxyVerification.LatencyMS
	}
	a.mu.Lock()
	a.qrSessions[img.Session.ID] = img.Session
	keep := make(map[string]bool, len(a.qrSessions))
	for sid := range a.qrSessions {
		keep[sid] = true
	}
	a.mu.Unlock()
	path := a.resources.qrPath(img.Session.ID)
	_ = os.WriteFile(path, img.ImageBytes, 0o644)
	a.cleanupQR(keep)
	out := map[string]any{
		"session_id":       img.Session.ID,
		"status":           img.Session.Status,
		"image_url":        "/qr/" + img.Session.ID + "/image",
		"proxy_enabled":    useProxy,
		"proxy_area":       stringPtrMaybe(areaCode),
		"proxy_latency_ms": proxyLatencyMS,
		"expires_in":       expiresIn,
	}
	if wantBase64 {
		out["image_base64"] = qr.DataURIJPEG(img.ImageBytes)
	} else {
		out["image_base64"] = nil
	}
	return out, http.StatusOK, nil
}

type qrCreateRequest struct {
	UseProxy bool   `json:"use_proxy"`
	Area     string `json:"area"`
}

func (a *App) handleQR(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/qr/"), "/")
	if len(parts) != 2 {
		writeError(w, http.StatusNotFound, "qr session not found")
		return
	}
	sessionID, action := parts[0], parts[1]
	switch action {
	case "image":
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		path := a.resources.qrPath(sessionID)
		if _, err := os.Stat(path); err != nil {
			writeError(w, http.StatusNotFound, "qr session not found")
			return
		}
		w.Header().Set("Content-Type", "image/jpeg")
		http.ServeFile(w, r, path)
	case "poll":
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		sess := a.getQRSession(sessionID)
		if sess == nil {
			writeError(w, http.StatusNotFound, "qr session not found")
			return
		}
		if sess.Expired() {
			a.dropQRSession(sessionID)
			writeJSON(w, http.StatusOK, qr.PollResult{Status: "expired"})
			return
		}
		result, err := a.qr.PollQRCode(r.Context(), sess)
		if err != nil {
			writeError(w, http.StatusBadGateway, err.Error())
			return
		}
		if terminalQR(result.Status) {
			a.dropQRSession(sessionID)
		}
		writeJSON(w, http.StatusOK, result)
	case "confirm":
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		sess := a.getQRSession(sessionID)
		if sess == nil {
			writeError(w, http.StatusNotFound, "qr session not found")
			return
		}
		if sess.Expired() {
			a.dropQRSession(sessionID)
			writeError(w, http.StatusGone, "qr session expired")
			return
		}
		result, err := a.qr.GetLoginBuffer(r.Context(), sess)
		if err != nil {
			writeError(w, http.StatusConflict, "buffer not ready: "+err.Error())
			return
		}
		var userInfo map[string]any
		if ui, err := a.qr.LoginBuffers().FetchUserInfo(r.Context(), result.Credentials); err == nil {
			userInfo = ui
		}
		acc, err := a.storeFromScan(r.Context(), result.LoginBuffer, result.Credentials, userInfo)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		a.dropQRSession(sessionID)
		writeJSON(w, http.StatusOK, acc.Public())
	default:
		writeError(w, http.StatusNotFound, "qr session not found")
	}
}

func (a *App) handleAccountsRoot(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/accounts" {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	switch r.Method {
	case http.MethodGet:
		accounts, err := a.db.ListAccounts(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		out := make([]store.AccountPublic, 0, len(accounts))
		for _, acc := range accounts {
			out = append(out, acc.Public())
		}
		writeJSON(w, http.StatusOK, out)
	case http.MethodDelete:
		acc, ok := a.resolveAccountFromQuery(w, r)
		if !ok {
			return
		}
		if err := a.db.DeleteAccount(r.Context(), acc.ID); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"deleted": acc.ID, "openid": acc.OpenID})
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (a *App) handleAccountAvatar(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/accounts/avatar" {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	acc, ok := a.resolveAccountFromQuery(w, r)
	if !ok {
		return
	}
	a.serveAvatar(w, r, acc)
}

func (a *App) handleAccountRefresh(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/accounts/refresh" {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var body accountRefIn
	if err := decodeOptionalJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if body.Ref == "" {
		a.refreshAll(w, r)
		return
	}
	acc, ok := a.resolveAccountRef(w, r, body.Ref)
	if !ok {
		return
	}
	status := a.refreshLiveness(r.Context(), acc)
	writeJSON(w, http.StatusOK, refreshOut(acc, status))
}

func (a *App) handleAccountResync(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/accounts/resync" {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var body accountRefIn
	if err := decodeOptionalJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if body.Ref == "" {
		a.resyncAll(w, r)
		return
	}
	acc, ok := a.resolveAccountRef(w, r, body.Ref)
	if !ok {
		return
	}
	updated, err := a.resyncProfile(r.Context(), acc)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, updated.Public())
}

func (a *App) handleGetCode(w http.ResponseWriter, r *http.Request) {
	if !acceptWXAppRoute(w, r, "/wxapp/getCode") {
		return
	}
	a.callWXApp(w, r, false, a.invokeGetCode)
}

func (a *App) handleGetPhoneNumber(w http.ResponseWriter, r *http.Request) {
	if !acceptWXAppRoute(w, r, "/wxapp/getPhoneNumber") {
		return
	}
	a.callWXApp(w, r, false, a.invokeGetPhoneNumber)
}

func (a *App) handleOperateWXData(w http.ResponseWriter, r *http.Request) {
	if !acceptWXAppRoute(w, r, "/wxapp/operateWxData") {
		return
	}
	a.callWXApp(w, r, true, a.invokeOperateWXData)
}

func acceptWXAppRoute(w http.ResponseWriter, r *http.Request, path string) bool {
	if r.URL.Path != path {
		writeError(w, http.StatusNotFound, "not found")
		return false
	}
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return false
	}
	return true
}

type accountRefIn struct {
	Ref string `json:"ref"`
}

type wxappRequest struct {
	Ref     string         `json:"ref"`
	AppID   string         `json:"app_id"`
	Payload map[string]any `json:"payload"`
	Proxy   string         `json:"proxy"` // 可选：本次调用使用的代理地址，或代理库 id（配合 ?proxy_id 亦可）
}

type wxappCall func(ctx context.Context, acc *store.WechatAccount, appID string, payload map[string]any, tcpProxy string) (map[string]any, error)

func (a *App) callWXApp(w http.ResponseWriter, r *http.Request, requirePayload bool, call wxappCall) {
	var body wxappRequest
	if err := decodeOptionalJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if body.Ref == "" {
		writeError(w, http.StatusBadRequest, "ref is required")
		return
	}
	if body.AppID == "" {
		writeError(w, http.StatusBadRequest, "app_id is required")
		return
	}
	if requirePayload && body.Payload == nil {
		writeError(w, http.StatusBadRequest, "payload is required")
		return
	}
	acc, ok := a.resolveAccountRef(w, r, body.Ref)
	if !ok {
		return
	}
	tcpProxy, perr := a.resolveTCPProxy(r, body.Proxy)
	if perr != nil {
		writeError(w, http.StatusBadRequest, perr.Error())
		return
	}
	result, err := a.invokeWXApp(r.Context(), acc, body.AppID, body.Payload, tcpProxy, call)
	if err != nil {
		var expired accountExpiredError
		switch {
		case errors.As(err, &expired):
			writeError(w, http.StatusConflict, "account login_buffer expired (refresh failed); re-scan required")
		default:
			writeError(w, http.StatusBadGateway, "call failed: "+err.Error())
		}
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"openid": acc.OpenID, "result": result})
}

func decodeOptionalJSON(r *http.Request, dst any) error {
	err := json.NewDecoder(r.Body).Decode(dst)
	if errors.Is(err, io.EOF) {
		return nil
	}
	return err
}

func (a *App) resolveAccountFromQuery(w http.ResponseWriter, r *http.Request) (*store.WechatAccount, bool) {
	ref := strings.TrimSpace(r.URL.Query().Get("ref"))
	if ref == "" {
		writeError(w, http.StatusBadRequest, "ref query param is required")
		return nil, false
	}
	return a.resolveAccountRef(w, r, ref)
}

func (a *App) resolveAccountRef(w http.ResponseWriter, r *http.Request, ref string) (*store.WechatAccount, bool) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		writeError(w, http.StatusBadRequest, "ref is required")
		return nil, false
	}
	acc, err := a.db.ResolveAccount(r.Context(), ref)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "account not found: "+ref)
		} else {
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return nil, false
	}
	return acc, true
}

func (a *App) refreshAll(w http.ResponseWriter, r *http.Request) {
	accounts, err := a.db.ListAccounts(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	out := make([]map[string]any, 0, len(accounts))
	for _, acc := range accounts {
		out = append(out, refreshOut(acc, a.refreshLiveness(r.Context(), acc)))
	}
	writeJSON(w, http.StatusOK, out)
}

func (a *App) resyncAll(w http.ResponseWriter, r *http.Request) {
	accounts, err := a.db.ListAccounts(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	out := make([]store.AccountPublic, 0, len(accounts))
	for _, acc := range accounts {
		updated, err := a.resyncProfile(r.Context(), acc)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		out = append(out, updated.Public())
	}
	writeJSON(w, http.StatusOK, out)
}

func (a *App) serveAvatar(w http.ResponseWriter, r *http.Request, acc *store.WechatAccount) {
	if acc.Avatar != nil && *acc.Avatar != "" {
		if _, err := os.Stat(*acc.Avatar); err == nil {
			w.Header().Set("Content-Type", "image/jpeg")
			http.ServeFile(w, r, *acc.Avatar)
			return
		}
		if strings.HasPrefix(*acc.Avatar, "http://") || strings.HasPrefix(*acc.Avatar, "https://") {
			http.Redirect(w, r, *acc.Avatar, http.StatusFound)
			return
		}
	}
	writeError(w, http.StatusNotFound, "no avatar")
}

func (a *App) storeFromScan(ctx context.Context, loginBuffer string, creds protocol.LoginBufferCredentials, userInfo map[string]any) (*store.WechatAccount, error) {
	openid := creds.OpenID
	nick := pickNickname(userInfo, creds.Nickname)
	avatar := a.resolveAvatar(ctx, openid, userInfo)
	status := "alive"
	return a.db.UpsertAccount(ctx, openid, loginBuffer, stringPtrMaybe(nick), stringPtrMaybe(nick), stringPtrMaybe(avatar), userInfo, creds.ToMap(), &status)
}

func (a *App) refreshLiveness(ctx context.Context, acc *store.WechatAccount) string {
	if acc.Credentials == nil {
		_ = a.db.SetAccountStatus(ctx, acc.ID, "unknown")
		return "unknown"
	}
	creds := protocol.CredentialsFromMap(acc.Credentials)
	result, err := a.qr.RefreshLoginBuffer(ctx, creds)
	if err != nil {
		_ = a.db.SetAccountStatus(ctx, acc.ID, "expired")
		return "expired"
	}
	_ = a.db.SetAccountCredential(ctx, acc.ID, result.LoginBuffer, result.Credentials.ToMap())
	_ = a.db.SetAccountStatus(ctx, acc.ID, "alive")
	if avatar := a.resolveAvatar(ctx, acc.OpenID, acc.UserInfo); avatar != "" {
		_ = a.db.SetAccountProfile(ctx, acc.ID, acc.Nickname, &avatar, acc.UserInfo)
	}
	return "alive"
}

func (a *App) resyncProfile(ctx context.Context, acc *store.WechatAccount) (*store.WechatAccount, error) {
	nick := pickNickname(acc.UserInfo, deref(acc.Nickname))
	avatar := a.resolveAvatar(ctx, acc.OpenID, acc.UserInfo)
	if avatar == "" {
		avatar = deref(acc.Avatar)
	}
	if err := a.db.SetAccountProfile(ctx, acc.ID, stringPtrMaybe(nick), stringPtrMaybe(avatar), acc.UserInfo); err != nil {
		return nil, err
	}
	return a.db.GetAccount(ctx, acc.ID)
}

type accountExpiredError struct{ openid string }

func (e accountExpiredError) Error() string { return "account expired: " + e.openid }

func (a *App) invokeWXApp(ctx context.Context, acc *store.WechatAccount, appID string, payload map[string]any, tcpProxy string, call wxappCall) (map[string]any, error) {
	if _, err := a.db.GetSession(ctx, acc.ID, tcpProxy); err == nil {
		result, err := call(ctx, acc, appID, payload, tcpProxy)
		if err == nil {
			return result, nil
		}
		_ = a.db.InvalidateSession(ctx, acc.ID, tcpProxy)
	}
	status := a.refreshLiveness(ctx, acc)
	if status != "alive" {
		return nil, accountExpiredError{openid: acc.OpenID}
	}
	fresh, err := a.db.GetAccount(ctx, acc.ID)
	if err == nil && fresh != nil {
		acc = fresh
	}
	return call(ctx, acc, appID, payload, tcpProxy)
}

func (a *App) invokeGetCode(ctx context.Context, acc *store.WechatAccount, appID string, _ map[string]any, tcpProxy string) (map[string]any, error) {
	return a.pool.GetCode(ctx, acc.LoginBuffer, appID, acc.ID, tcpProxy)
}

func (a *App) invokeGetPhoneNumber(ctx context.Context, acc *store.WechatAccount, appID string, _ map[string]any, tcpProxy string) (map[string]any, error) {
	return a.pool.GetPhoneNumber(ctx, acc.LoginBuffer, appID, acc.ID, tcpProxy)
}

func (a *App) invokeOperateWXData(ctx context.Context, acc *store.WechatAccount, appID string, payload map[string]any, tcpProxy string) (map[string]any, error) {
	return a.pool.OperateWXData(ctx, acc.LoginBuffer, appID, payload, acc.ID, tcpProxy)
}

func refreshOut(acc *store.WechatAccount, status string) map[string]any {
	return map[string]any{"id": acc.ID, "openid": acc.OpenID, "uin": acc.UIN, "nickname": acc.Nickname, "status": status}
}

func pickNickname(userInfo map[string]any, fallback string) string {
	if s := stringFromAny(userInfo["nick_name"]); s != "" {
		return s
	}
	return fallback
}

func pickAvatarURL(userInfo map[string]any) string {
	for _, k := range []string{"head_img_url", "head_url", "headimgurl", "avatar"} {
		if s := stringFromAny(userInfo[k]); s != "" {
			return s
		}
	}
	return ""
}

func (a *App) resolveAvatar(ctx context.Context, openid string, userInfo map[string]any) string {
	u := pickAvatarURL(userInfo)
	if u == "" {
		return ""
	}
	dest := a.resources.avatarPath(openid)
	if downloadAvatar(ctx, u, dest, a.cfg.AvatarTimeout) {
		return dest
	}
	return u
}

func downloadAvatar(ctx context.Context, url, dest string, timeout time.Duration) bool {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return false
	}
	client := &http.Client{Timeout: timeout}
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil || resp.StatusCode != 200 || !looksLikeImage(data) {
		return false
	}
	_ = os.MkdirAll(filepath.Dir(dest), 0o755)
	return os.WriteFile(dest, data, 0o644) == nil
}

func looksLikeImage(data []byte) bool {
	if len(data) < 64 {
		return false
	}
	magics := [][]byte{{0xff, 0xd8, 0xff}, {0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}, []byte("GIF87a"), []byte("GIF89a")}
	for _, m := range magics {
		if strings.HasPrefix(string(data), string(m)) {
			return true
		}
	}
	return false
}

func (a *App) getQRSession(id string) *qr.Session {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.qrSessions[id]
}

func (a *App) dropQRSession(id string) {
	a.mu.Lock()
	sess := a.qrSessions[id]
	delete(a.qrSessions, id)
	a.mu.Unlock()
	if sess != nil {
		sess.Close()
	}
	_ = os.Remove(a.resources.qrPath(id))
}

func (a *App) pruneQR() {
	a.mu.Lock()
	var drop []string
	for sid, sess := range a.qrSessions {
		if sess.Expired() || sess.Age() > a.cfg.QRSessionTTL {
			drop = append(drop, sid)
		}
	}
	for _, sid := range drop {
		a.qrSessions[sid].Close()
		delete(a.qrSessions, sid)
	}
	a.mu.Unlock()
	for _, sid := range drop {
		_ = os.Remove(a.resources.qrPath(sid))
	}
}

func secondsUntil(deadline time.Time) int {
	remaining := time.Until(deadline)
	if remaining <= 0 {
		return 0
	}
	return int((remaining + time.Second - 1) / time.Second)
}

func (a *App) cleanupQR(keep map[string]bool) {
	files, _ := filepath.Glob(filepath.Join(a.resources.QR, "*.jpg"))
	for _, f := range files {
		sid := strings.TrimSuffix(filepath.Base(f), ".jpg")
		if !keep[sid] {
			_ = os.Remove(f)
		}
	}
}

func terminalQR(status string) bool {
	return status == "expired" || status == "cancelled" || status == "unknown"
}

type apiEnvelope struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data any    `json:"data"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	writeRawJSON(w, status, apiEnvelope{
		Code: 0,
		Msg:  "success",
		Data: v,
	})
}

func writeRawJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(v)
}

func writeError(w http.ResponseWriter, status int, detail string) {
	writeRawJSON(w, status, apiEnvelope{
		Code: status,
		Msg:  detail,
		Data: nil,
	})
}

func requestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r)
	})
}

func serveFileOrText(w http.ResponseWriter, r *http.Request, path, fallback string) {
	if _, err := os.Stat(path); err == nil {
		http.ServeFile(w, r, path)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(fallback))
}

func serveHTML(w http.ResponseWriter, page string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(page))
}

func stringFromAny(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func stringPtrMaybe(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func safeName(s string) string {
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func sortedKeys[M ~map[string]V, V any](m M) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
