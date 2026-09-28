package pinzan

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	defaultExtractURL = "https://service.ipzan.com/core-extract"
	defaultHealthURL  = "https://open.weixin.qq.com/"
	maxCandidates     = 3
)

type Config struct {
	No         string
	Secret     string
	Minute     int
	Timeout    time.Duration
	ExtractURL string
	HealthURL  string
}

type Client struct {
	cfg    Config
	direct *http.Client
}

type Verification struct {
	LatencyMS int64
	ExpiresAt time.Time
}

func NewClient(cfg Config) *Client {
	if cfg.Minute == 0 {
		cfg.Minute = 1
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 8 * time.Second
	}
	if cfg.ExtractURL == "" {
		cfg.ExtractURL = defaultExtractURL
	}
	if cfg.HealthURL == "" {
		cfg.HealthURL = defaultHealthURL
	}
	directTransport := http.DefaultTransport.(*http.Transport).Clone()
	directTransport.Proxy = nil
	return &Client{
		cfg: cfg,
		direct: &http.Client{
			Timeout:   cfg.Timeout,
			Transport: directTransport,
		},
	}
}

func (c *Client) Configured() bool {
	return strings.TrimSpace(c.cfg.No) != "" && strings.TrimSpace(c.cfg.Secret) != ""
}

func (c *Client) NewHTTPClient(ctx context.Context, area string, timeout time.Duration) (*http.Client, Verification, error) {
	if !c.Configured() {
		return nil, Verification{}, errors.New("品赞代理未配置，请设置 YYB_PINZAN_NO 和 YYB_PINZAN_SECRET")
	}
	if !validMinute(c.cfg.Minute) {
		return nil, Verification{}, fmt.Errorf("品赞代理时长 %d 无效", c.cfg.Minute)
	}
	if timeout <= 0 {
		timeout = c.cfg.Timeout
	}

	var lastHealthErr error
	for attempt := 1; attempt <= maxCandidates; attempt++ {
		area = strings.TrimSpace(area)
		if area == "" {
			area = "all"
		}
		extractedAt := time.Now()
		proxyURL, err := c.extract(ctx, area)
		if err != nil {
			return nil, Verification{}, err
		}
		transport := http.DefaultTransport.(*http.Transport).Clone()
		transport.Proxy = http.ProxyURL(proxyURL)
		client := &http.Client{Timeout: timeout, Transport: transport}
		checkStarted := time.Now()
		if err := c.checkHealth(ctx, client); err == nil {
			latencyMS := time.Since(checkStarted).Milliseconds()
			if latencyMS < 1 {
				latencyMS = 1
			}
			return client, Verification{
				LatencyMS: latencyMS,
				ExpiresAt: extractedAt.Add(time.Duration(c.cfg.Minute) * time.Minute),
			}, nil
		} else {
			lastHealthErr = err
			transport.CloseIdleConnections()
		}
	}
	return nil, Verification{}, fmt.Errorf("品赞代理连续 %d 次健康检查失败: %w", maxCandidates, lastHealthErr)
}

func (c *Client) extract(ctx context.Context, area string) (*url.URL, error) {
	u, err := url.Parse(c.cfg.ExtractURL)
	if err != nil {
		return nil, fmt.Errorf("品赞提取地址无效: %w", err)
	}
	query := u.Query()
	query.Set("no", c.cfg.No)
	query.Set("secret", c.cfg.Secret)
	query.Set("num", "1")
	query.Set("mode", "whitelist")
	query.Set("format", "json")
	query.Set("protocol", "1")
	query.Set("pool", "quality")
	query.Set("minute", strconv.Itoa(c.cfg.Minute))
	if area != "" {
		query.Set("area", area)
	}
	u.RawQuery = query.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("创建品赞提取请求失败: %w", err)
	}
	resp, err := c.direct.Do(req)
	if err != nil {
		return nil, fmt.Errorf("品赞代理提取失败: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("读取品赞响应失败: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("品赞代理提取 HTTP %d", resp.StatusCode)
	}

	var result extractResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("品赞代理响应不是有效 JSON")
	}
	if result.Code != 0 {
		message := redactProviderMessage(result.ProviderMessage(), c.cfg.No, c.cfg.Secret)
		if message == "" {
			message = fmt.Sprintf("业务码 %d（响应未提供错误说明）", result.Code)
		}
		return nil, fmt.Errorf("品赞代理提取失败: %s", message)
	}
	if len(result.Data.List) == 0 {
		return nil, errors.New("品赞代理提取失败: 返回列表为空")
	}
	host := strings.TrimSpace(result.Data.List[0].Host)
	if host == "" {
		host = strings.TrimSpace(result.Data.List[0].IP)
	}
	port, err := result.Data.List[0].Port.Int()
	if host == "" || err != nil || port < 1 || port > 65535 {
		return nil, errors.New("品赞代理提取失败: 返回的地址或端口无效")
	}
	return url.Parse("http://" + net.JoinHostPort(host, strconv.Itoa(port)))
}

func (c *Client) checkHealth(ctx context.Context, client *http.Client) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.cfg.HealthURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusBadRequest {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return nil
}

func validMinute(value int) bool {
	switch value {
	case 1, 3, 5, 10, 15, 30:
		return true
	default:
		return false
	}
}

type extractResponse struct {
	Code             int         `json:"code"`
	Msg              string      `json:"msg"`
	Message          string      `json:"message"`
	Error            string      `json:"error"`
	ErrorMessage     string      `json:"error_message"`
	ErrorDescription string      `json:"error_description"`
	Info             string      `json:"info"`
	Reason           string      `json:"reason"`
	Data             extractData `json:"data"`
}

type extractData struct {
	List []struct {
		IP   string      `json:"ip"`
		Host string      `json:"host"`
		Port numberValue `json:"port"`
	} `json:"list"`
	Msg              string `json:"msg"`
	Message          string `json:"message"`
	Error            string `json:"error"`
	ErrorMessage     string `json:"error_message"`
	ErrorDescription string `json:"error_description"`
	Info             string `json:"info"`
	Reason           string `json:"reason"`
	Text             string `json:"-"`
}

func (r extractResponse) ProviderMessage() string {
	for _, value := range []string{
		r.Msg, r.Message, r.Error, r.ErrorMessage, r.ErrorDescription, r.Info, r.Reason,
		r.Data.Msg, r.Data.Message, r.Data.Error, r.Data.ErrorMessage,
		r.Data.ErrorDescription, r.Data.Info, r.Data.Reason, r.Data.Text,
	} {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}

func (d *extractData) UnmarshalJSON(data []byte) error {
	data = bytes.TrimSpace(data)
	if len(data) == 0 || bytes.Equal(data, []byte("null")) {
		return nil
	}
	if data[0] == '"' {
		return json.Unmarshal(data, &d.Text)
	}
	if data[0] != '{' {
		return nil
	}
	type plain extractData
	return json.Unmarshal(data, (*plain)(d))
}

func redactProviderMessage(message string, sensitive ...string) string {
	message = strings.TrimSpace(message)
	for _, value := range sensitive {
		if value = strings.TrimSpace(value); value != "" {
			message = strings.ReplaceAll(message, value, "***")
		}
	}
	return message
}

type numberValue string

func (v *numberValue) UnmarshalJSON(data []byte) error {
	var text string
	if len(data) > 0 && data[0] == '"' {
		if err := json.Unmarshal(data, &text); err != nil {
			return err
		}
	} else {
		text = string(data)
	}
	*v = numberValue(text)
	return nil
}

func (v numberValue) Int() (int, error) {
	return strconv.Atoi(string(v))
}
