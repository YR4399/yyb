package httpapi

import (
	"errors"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"yyb_go/internal/qr"
	"yyb_go/internal/store"
)

// ---------- 账号：WCS 兼容的 /api/accounts/* ----------

// handleAPIAccounts GET 列出全部账号（与 /accounts 等价）。
func (a *App) handleAPIAccounts(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/api/accounts" {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
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
}

type accountAddRequest struct {
	LoginBuffer string  `json:"login_buffer"`
	OpenID      string  `json:"openid"`
	Nickname    *string `json:"nickname"`
	Alias       *string `json:"alias"`
}

// handleAccountAdd 通过 login_buffer 导入一个已有账号（openid 必填）。
func (a *App) handleAccountAdd(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/api/accounts/add" {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var body accountAddRequest
	if err := decodeOptionalJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	body.LoginBuffer = strings.TrimSpace(body.LoginBuffer)
	body.OpenID = strings.TrimSpace(body.OpenID)
	if body.LoginBuffer == "" {
		writeError(w, http.StatusBadRequest, "login_buffer is required")
		return
	}
	if body.OpenID == "" {
		writeError(w, http.StatusBadRequest, "openid is required (扫码登录请用 POST /qr)")
		return
	}
	status := "imported"
	acc, err := a.db.UpsertAccount(r.Context(), body.OpenID, body.LoginBuffer,
		body.Alias, body.Nickname, nil, nil, nil, &status)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, acc.Public())
}

// handleAccountDeleteAPI 删除账号（body.ref）。
func (a *App) handleAccountDeleteAPI(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/api/accounts/delete" {
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
	acc, ok := a.resolveAccountFromQueryLike(w, r, body.Ref)
	if !ok {
		return
	}
	if err := a.db.DeleteAccount(r.Context(), acc.ID); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"deleted": acc.ID, "openid": acc.OpenID})
}

// handleAccountDisable 启用/禁用账号（body.ref, body.disabled）。
func (a *App) handleAccountDisable(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/api/accounts/disable" {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var body struct {
		Ref      string `json:"ref"`
		Disabled *bool  `json:"disabled"`
	}
	if err := decodeOptionalJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	acc, ok := a.resolveAccountFromQueryLike(w, r, body.Ref)
	if !ok {
		return
	}
	next := "alive"
	if body.Disabled != nil && *body.Disabled {
		next = "disabled"
	}
	if err := a.db.SetAccountStatus(r.Context(), acc.ID, next); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": acc.ID, "openid": acc.OpenID, "status": next})
}

// handleAccountRemark 设置账号备注/别名（body.ref, body.remark）。
func (a *App) handleAccountRemark(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/api/accounts/remark" {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var body struct {
		Ref    string  `json:"ref"`
		Remark *string `json:"remark"`
	}
	if err := decodeOptionalJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if body.Remark == nil {
		writeError(w, http.StatusBadRequest, "remark is required")
		return
	}
	acc, ok := a.resolveAccountFromQueryLike(w, r, body.Ref)
	if !ok {
		return
	}
	if err := a.db.SetAccountProfile(r.Context(), acc.ID, body.Remark, acc.Avatar, acc.UserInfo); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": acc.ID, "openid": acc.OpenID, "alias": *body.Remark})
}

// handleAccountRescanAPI 重新校验/刷新账号登录态（body.ref）。
func (a *App) handleAccountRescanAPI(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/api/accounts/rescan" {
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
	acc, ok := a.resolveAccountFromQueryLike(w, r, body.Ref)
	if !ok {
		return
	}
	status := a.refreshLiveness(r.Context(), acc)
	writeJSON(w, http.StatusOK, refreshOut(acc, status))
}

// handleAccountStatus 返回单个账号的状态（query.ref）。
func (a *App) handleAccountStatus(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/api/accounts/status" {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	acc, ok := a.resolveAccountFromQueryLike(w, r, "")
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"id":              acc.ID,
		"openid":          acc.OpenID,
		"uin":             acc.UIN,
		"nickname":        acc.Nickname,
		"status":          acc.Status,
		"last_checked_at": acc.LastCheckedAt,
	})
}

// resolveAccountFromQueryLike 同时支持 query.ref 与 body.ref（WCS 习惯用 body）。
func (a *App) resolveAccountFromQueryLike(w http.ResponseWriter, r *http.Request, bodyRef string) (*store.WechatAccount, bool) {
	ref := strings.TrimSpace(r.URL.Query().Get("ref"))
	if ref == "" {
		ref = strings.TrimSpace(bodyRef)
	}
	if ref == "" {
		writeError(w, http.StatusBadRequest, "ref is required")
		return nil, false
	}
	return a.resolveAccountRef(w, r, ref)
}

// ---------- 扫码：WCS 兼容的 /api/qr/* ----------

// handleAPIQRStart 创建扫码登录会话（镜像 /qr）。
func (a *App) handleAPIQRStart(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/api/qr/start" {
		writeError(w, http.StatusNotFound, "not found")
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

// handleAPIQRStatus 轮询扫码状态（query.session_id）。
func (a *App) handleAPIQRStatus(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/api/qr/status" {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	sessionID := strings.TrimSpace(r.URL.Query().Get("session_id"))
	if sessionID == "" {
		writeError(w, http.StatusBadRequest, "session_id is required")
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
}

// ---------- 鉴权：WCS 兼容的 /api/auth/validate ----------

type authValidateRequest struct {
	Token string `json:"token"`
}

// handleAuthValidate 校验调用方令牌；未配置 YYB_API_TOKEN 时视为开放（始终有效）。
func (a *App) handleAuthValidate(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/api/auth/validate" {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var body authValidateRequest
	if err := decodeOptionalJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if a.cfg.APIToken == "" {
		writeJSON(w, http.StatusOK, map[string]any{"valid": true, "note": "server has no API token configured (open mode)"})
		return
	}
	valid := body.Token == a.cfg.APIToken
	writeJSON(w, http.StatusOK, map[string]any{"valid": valid})
}

// ---------- 代理：WCS 兼容的 /api/proxies/* ----------

// handleProxiesList GET 列出全部代理。
func (a *App) handleProxiesList(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/api/proxies" {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	proxies, err := a.db.ListProxies(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, proxies)
}

type proxyAddRequest struct {
	Scheme   string  `json:"scheme"`
	Host     string  `json:"host"`
	Port     int     `json:"port"`
	Username *string `json:"username"`
	Password *string `json:"password"`
	Note     *string `json:"note"`
	Enabled  *bool   `json:"enabled"`
}

// handleProxyAdd POST 新增一条代理。
func (a *App) handleProxyAdd(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/api/proxies/add" {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var body proxyAddRequest
	if err := decodeOptionalJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	body.Host = strings.TrimSpace(body.Host)
	body.Scheme = strings.TrimSpace(body.Scheme)
	if body.Host == "" || body.Port <= 0 {
		writeError(w, http.StatusBadRequest, "host and port(>0) are required")
		return
	}
	enabled := true
	if body.Enabled != nil {
		enabled = *body.Enabled
	}
	p, err := a.db.AddProxy(r.Context(), body.Scheme, body.Host, body.Port, body.Username, body.Password, body.Note, enabled)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, p)
}

// handleProxyDelete POST/DELETE 删除代理（body.id）。
func (a *App) handleProxyDelete(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/api/proxies/delete" {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	if r.Method != http.MethodPost && r.Method != http.MethodDelete {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var body struct {
		ID int64 `json:"id"`
	}
	if err := decodeOptionalJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if body.ID <= 0 {
		writeError(w, http.StatusBadRequest, "id is required")
		return
	}
	if err := a.db.DeleteProxy(r.Context(), body.ID); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"deleted": body.ID})
}

type proxyTestRequest struct {
	ID   int64  `json:"id"`
	Host string `json:"host"`
	Port int    `json:"port"`
}

// handleProxyTest POST 测试代理连通性（按 id 或 host:port）。
func (a *App) handleProxyTest(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/api/proxies/test" {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var body proxyTestRequest
	if err := decodeOptionalJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	host := body.Host
	port := body.Port
	if body.ID > 0 {
		p, err := a.db.GetProxy(r.Context(), body.ID)
		if err != nil {
			writeError(w, http.StatusNotFound, "proxy not found: "+err.Error())
			return
		}
		host = p.Host
		port = p.Port
	}
	if host == "" || port <= 0 {
		writeError(w, http.StatusBadRequest, "id or host+port is required")
		return
	}
	timeout := a.cfg.ProxyTestTimeout
	if timeout == 0 {
		timeout = 5 * time.Second
	}
	addr := net.JoinHostPort(host, strconv.Itoa(port))
	start := time.Now()
	conn, err := net.DialTimeout("tcp", addr, timeout)
	latency := time.Since(start).Milliseconds()
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "latency_ms": latency, "error": err.Error(), "addr": addr})
		return
	}
	_ = conn.Close()
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "latency_ms": latency, "addr": addr})
}

// ---------- wx 业务：WCS 兼容的 /wx/* ----------

func (a *App) handleWXCode(w http.ResponseWriter, r *http.Request) {
	if !acceptWXRoute(w, r, "/wx/code") {
		return
	}
	a.callWXApp(w, r, false, a.invokeGetCode)
}

func (a *App) handleWXGetPhoneNumber(w http.ResponseWriter, r *http.Request) {
	if !acceptWXRoute(w, r, "/wx/getphonenumber") {
		return
	}
	a.callWXApp(w, r, false, a.invokeGetPhoneNumber)
}

func (a *App) handleWXOperate(w http.ResponseWriter, r *http.Request) {
	if !acceptWXRoute(w, r, "/wx/operateWxData") {
		return
	}
	a.callWXApp(w, r, true, a.invokeOperateWXData)
}

func acceptWXRoute(w http.ResponseWriter, r *http.Request, path string) bool {
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

// handleWXGetUserInfo 返回已保存账号的用户资料（store 支持）。
func (a *App) handleWXGetUserInfo(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/wx/getuserinfo" {
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
	acc, ok := a.resolveAccountFromQueryLike(w, r, body.Ref)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"openid":   acc.OpenID,
		"nickname": acc.Nickname,
		"avatar":   acc.Avatar,
		"user_info": acc.UserInfo,
	})
}

// handleWXGetSession 返回账号协议会话状态（store 支持）。
func (a *App) handleWXGetSession(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/wx/getsession" {
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
	acc, ok := a.resolveAccountFromQueryLike(w, r, body.Ref)
	if !ok {
		return
	}
	proxy, _ := a.resolveTCPProxy(r, "")
	state := "expired"
	if _, err := a.db.GetSession(r.Context(), acc.ID, proxy); err == nil {
		state = "alive"
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"openid": acc.OpenID,
		"session": state,
		"uin":     acc.UIN,
	})
}

// handleWXRefresh 刷新并保存账号登录态（store + 协议层支持）。
func (a *App) handleWXRefresh(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/wx/refresh" {
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
	acc, ok := a.resolveAccountFromQueryLike(w, r, body.Ref)
	if !ok {
		return
	}
	status := a.refreshLiveness(r.Context(), acc)
	writeJSON(w, http.StatusOK, refreshOut(acc, status))
}

// handleWXUnsupported WCS 的 /wx/* 兜底：列出 yyb 核心已支持的 wx 操作，
// 未由 yyb 协议层实现的接口（oauth/cloud/gateway/translatelink 等）返回清晰说明，避免伪造。
func (a *App) handleWXUnsupported(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/wx/")
	writeError(w, http.StatusNotImplemented, "yyb core protocol does not implement /wx/"+path+
		"; supported: /wx/code, /wx/getphonenumber, /wx/operateWxData, /wx/getuserinfo, /wx/getsession, /wx/refresh")
}

// resolveTCPProxy 解析本次 wx 调用使用的 TCP 代理：
// 1) ?proxy_id= 查代理库；2) ?proxy= 或 body.proxy 直接给地址（无 scheme 时默认 socks5）；
// 3) 否则回退到服务启动时配置的 -tcp-proxy。
func (a *App) resolveTCPProxy(r *http.Request, bodyProxy string) (string, error) {
	if idStr := strings.TrimSpace(r.URL.Query().Get("proxy_id")); idStr != "" {
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			return "", errors.New("proxy_id must be an integer")
		}
		p, err := a.db.GetProxy(r.Context(), id)
		if err != nil {
			return "", errors.New("proxy not found: " + idStr)
		}
		if !p.Enabled {
			return "", errors.New("proxy is disabled: " + idStr)
		}
		return p.ProxyAddr(), nil
	}
	addr := strings.TrimSpace(r.URL.Query().Get("proxy"))
	if addr == "" {
		addr = strings.TrimSpace(bodyProxy)
	}
	if addr == "" {
		return a.cfg.TCPProxy, nil
	}
	if !strings.Contains(addr, "://") {
		addr = "socks5://" + addr
	}
	return addr, nil
}
