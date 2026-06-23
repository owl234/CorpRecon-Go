package fetcher

import (
	"bytes"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"sync"
	"time"

	"github.com/sienchen/CorpRecon-Go/internal/auth"
)

// Client 封装了 HTTP 客户端，带有身份认证信息和防封禁机制
type Client struct {
	httpClient *http.Client
	session    *auth.SessionData
	
	// MaxRetries 请求失败时的最大重试次数
	MaxRetries int
	// MinDelay 每次请求的最小随机休眠时间
	MinDelay time.Duration
	// MaxDelay 每次请求的最大随机休眠时间
	MaxDelay time.Duration

	lastReqTime time.Time
	reqMutex    sync.Mutex
}

// NewClient 创建一个新的抓取客户端
func NewClient(session *auth.SessionData) *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
		session:     session,
		MaxRetries:  3,
		MinDelay:    500 * time.Millisecond,
		MaxDelay:    2000 * time.Millisecond,
		lastReqTime: time.Now(),
	}
}

// randomSleep 随机休眠以平滑并发请求（全局排队机制）
func (c *Client) randomSleep() {
	c.reqMutex.Lock()

	now := time.Now()
	elapsed := now.Sub(c.lastReqTime)

	var delay time.Duration
	if c.MaxDelay <= c.MinDelay {
		delay = c.MinDelay
	} else {
		delay = c.MinDelay + time.Duration(rand.Int63n(int64(c.MaxDelay-c.MinDelay)))
	}

	sleepTime := time.Duration(0)
	if elapsed < delay {
		sleepTime = delay - elapsed
	}

	// 提前设定下一个请求应该等待的时间点，实现无锁排队
	c.lastReqTime = now.Add(sleepTime)
	c.reqMutex.Unlock()

	if sleepTime > 0 {
		time.Sleep(sleepTime)
	}
}

// DoRequest 执行具体的 HTTP 请求，带有重试机制
func (c *Client) DoRequest(req *http.Request) ([]byte, error) {
	req.Header.Set("User-Agent", c.session.UserAgent)
	if c.session.Cookies != "" {
		req.Header.Set("Cookie", c.session.Cookies)
	}


	var lastErr error
	for i := 0; i < c.MaxRetries; i++ {
		c.randomSleep() // 请求前随机休眠

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("network error: %w", err)
			continue
		}
		
		defer resp.Body.Close()
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			lastErr = fmt.Errorf("read body error: %w", err)
			continue
		}

		// 处理常见的被反爬或封禁的状态码
		if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusTooManyRequests {
			lastErr = fmt.Errorf("blocked with status code: %d", resp.StatusCode)
			// 此时可以触发 Token Refresh (唤醒无头浏览器)，目前先简单报错并增加休眠
			time.Sleep(5 * time.Second)
			continue
		}

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			lastErr = fmt.Errorf("bad status code: %d, body: %s", resp.StatusCode, string(bodyBytes))
			fmt.Printf("DEBUG BAD STATUS: %d, body: %s\n", resp.StatusCode, string(bodyBytes))
			continue
		}

		return bodyBytes, nil
	}

	return nil, fmt.Errorf("max retries exceeded, last error: %v", lastErr)
}

// Get 发起 GET 请求
func (c *Client) Get(url string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	return c.DoRequest(req)
}

// PostJSON 发起携带自定义 Header 的 POST 请求
func (c *Client) PostJSON(url string, body []byte, headers map[string]string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	return c.DoRequest(req)
}
