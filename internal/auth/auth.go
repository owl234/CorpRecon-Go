package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
)

// BrowserConfig 配置无头浏览器自动登录的参数
type BrowserConfig struct {
	LoginURL string        // 登录页面的 URL (风鸟网主页 https://www.riskbird.com)
	Username string        // 账号
	Password string        // 密码
	Timeout  time.Duration // 整体登录超时时间
	Headless bool          // 是否无头模式（测试时可以设为 false 看界面）
}

// SessionData 包含了抓取层需要的身份凭证
type SessionData struct {
	Cookies   string
	UserAgent string
}

const CacheFile = ".session_cache.json"

// LoadSessionCache 从文件加载会话缓存
func LoadSessionCache(filePath string) (*SessionData, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	var session SessionData
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, err
	}
	return &session, nil
}

// SaveSessionCache 将会话保存到缓存文件
func SaveSessionCache(filePath string, session *SessionData) error {
	data, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filePath, data, 0644)
}

// ValidateSession 验证缓存的 Cookie 是否依然有效
func ValidateSession(session *SessionData) bool {
	urlStr := "https://www.riskbird.com/riskbird-api/api/v1/companies/search"
	payload := []byte(`{"queryType":"1","searchKey":"测试","pageNo":1,"range":10,"selectConditionData":"{\"status\":\"\",\"sort_field\":\"\",\"keyword_fields\":\"\"}"}`)
	req, err := http.NewRequest("POST", urlStr, bytes.NewBuffer(payload))
	if err != nil {
		return false
	}
	req.Header.Set("User-Agent", session.UserAgent)
	if session.Cookies != "" {
		req.Header.Set("Cookie", session.Cookies)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("accept", "application/json")
	req.Header.Set("app-device", "WEB")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return false
	}

	var result struct {
		Code int `json:"code"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return false
	}
	return result.Code == 20000
}

// GetSession 启动无头浏览器，执行自动登录，并提取 Cookies 和 UA
func GetSession(ctx context.Context, cfg BrowserConfig) (*SessionData, error) {
	log.Println("检查本地 Cookie 缓存...")
	if session, err := LoadSessionCache(CacheFile); err == nil {
		if ValidateSession(session) {
			log.Println("Cookie 有效，使用缓存！跳过登录。")
			return session, nil
		}
		log.Println("Cookie 缓存无效或已过期，准备重新获取...")
	} else {
		log.Println("未找到本地 Cookie 缓存，准备登录...")
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 60 * time.Second
	}

	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", cfg.Headless),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.UserAgent("Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"),
	)

	allocCtx, cancel := chromedp.NewExecAllocator(ctx, opts...)
	defer cancel()

	ctx, cancel = chromedp.NewContext(allocCtx, chromedp.WithLogf(log.Printf))
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, cfg.Timeout)
	defer cancel()

	var cookies []*network.Cookie
	var userAgent string

	log.Printf("Navigating to %s", cfg.LoginURL)

	err := chromedp.Run(ctx,
		// 1. 访问主页
		chromedp.Navigate(cfg.LoginURL),
		
		// 2. 等待弹窗出现，并点击右上角从“扫码”切换到“表单登录”
		chromedp.ActionFunc(func(ctx context.Context) error {
			log.Println("等待登录弹窗出现，并点击右上角切换为表单模式...")
			return nil
		}),
		chromedp.WaitVisible("img.Login-mode-img", chromedp.ByQuery),
		chromedp.Click("img.Login-mode-img", chromedp.ByQuery),

		// 3. 点击表单里的“密码登录” Tab，切换到账密输入模式
		chromedp.ActionFunc(func(ctx context.Context) error {
			log.Println("点击【密码登录】选项卡...")
			return nil
		}),
		chromedp.WaitVisible(`//div[@class='tab-item' and text()='密码登录']`, chromedp.BySearch),
		chromedp.Click(`//div[@class='tab-item' and text()='密码登录']`, chromedp.BySearch),

		// 4. 等待账号和密码输入框出现，并输入
		chromedp.ActionFunc(func(ctx context.Context) error {
			log.Println("正在输入账号和密码...")
			return nil
		}),
		chromedp.WaitVisible(`input[type='password']`, chromedp.ByQuery),
		chromedp.SetValue(`input[placeholder*='手机']`, cfg.Username, chromedp.ByQuery),
		chromedp.SetValue(`input[type='password']`, cfg.Password, chromedp.ByQuery),

		// 5. 点击登录按钮 (这里使用 button 的类名进行点击)
		chromedp.ActionFunc(func(ctx context.Context) error {
			log.Println("点击登录按钮...")
			return nil
		}),
		chromedp.Click(`button.login-form-item-btn`, chromedp.ByQuery),

		// 6. 留出充分的时间等待登录成功（处理接口请求、跳转或可能出现的人工滑块验证）
		chromedp.ActionFunc(func(ctx context.Context) error {
			log.Println("等待 10 秒钟，确保登录完成 (如果此时出现滑块验证，请在浏览器中手动拖动)...")
			return nil
		}),
		chromedp.Sleep(10 * time.Second),

		// 7. 执行 JS 获取当前的 User-Agent
		chromedp.Evaluate(`navigator.userAgent`, &userAgent),
		
		// 8. 获取网络层的 Cookies
		chromedp.ActionFunc(func(ctx context.Context) error {
			var err error
			cookies, err = network.GetCookies().Do(ctx)
			return err
		}),
	)

	if err != nil {
		return nil, fmt.Errorf("chromedp login failed: %w", err)
	}

	var cookieStrs []string
	for _, c := range cookies {
		cookieStrs = append(cookieStrs, fmt.Sprintf("%s=%s", c.Name, c.Value))
	}
	cookieStr := strings.Join(cookieStrs, "; ")

	session := &SessionData{
		Cookies:   cookieStr,
		UserAgent: userAgent,
	}

	if err := SaveSessionCache(CacheFile, session); err != nil {
		log.Printf("保存 Cookie 缓存失败: %v", err)
	}


	return session, nil
}

// SolveCaptchaInteractively 唤起有头浏览器，注入当前 Cookie，访问被拦截页面并轮询等待人工滑块，成功后更新 Cookie 缓存并返回 orderNo
func SolveCaptchaInteractively(targetURL string) (string, error) {
	session, err := LoadSessionCache(CacheFile)
	if err != nil {
		return "", fmt.Errorf("failed to load session for captcha solver: %w", err)
	}

	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", false), // 强制唤起有头 UI 供人工滑动
		chromedp.Flag("disable-gpu", false),
		chromedp.Flag("no-sandbox", true),
		chromedp.UserAgent(session.UserAgent),
	)

	allocCtx, cancelAlloc := chromedp.NewExecAllocator(context.Background(), opts...)
	defer cancelAlloc()

	ctx, cancelCtx := chromedp.NewContext(allocCtx, chromedp.WithLogf(log.Printf))
	defer cancelCtx()

	// 300秒的总超时时间，给用户留足时间排雷
	ctx, cancel := context.WithTimeout(ctx, 300*time.Second)
	defer cancel()

	var orderNo string

	err = chromedp.Run(ctx,
		// 先访问主域名，确立环境
		chromedp.Navigate("https://www.riskbird.com/"),
		// 将现有的缓存 Cookie 注进去
		chromedp.ActionFunc(func(ctx context.Context) error {
			cookies := strings.Split(session.Cookies, "; ")
			for _, c := range cookies {
				parts := strings.SplitN(c, "=", 2)
				if len(parts) == 2 {
					network.SetCookie(parts[0], parts[1]).WithDomain("www.riskbird.com").WithPath("/").Do(ctx)
					network.SetCookie(parts[0], parts[1]).WithDomain(".riskbird.com").WithPath("/").Do(ctx)
				}
			}
			return nil
		}),
		// 带着登录状态去访问真正的拦截页面
		chromedp.Navigate(targetURL),
		// 轮询等待 orderNo 出现 (证明滑块验证已经通过，页面完成跳转)
		chromedp.ActionFunc(func(ctx context.Context) error {
			re := regexp.MustCompile(`(WEB[0-9]{21})`)
			for {
				select {
				case <-ctx.Done():
					return ctx.Err()
				default:
					var htmlStr string
					// 提取页面源码
					if err := chromedp.OuterHTML("html", &htmlStr).Do(ctx); err == nil {
						matches := re.FindStringSubmatch(htmlStr)
						if len(matches) > 1 {
							orderNo = matches[1]
							return nil // 找到 orderNo，验证通过，退出轮询
						}
					}
					time.Sleep(2 * time.Second)
				}
			}
		}),
		// 如果验证通过，必定会发下新的 Cookie (解封 IP)，将其更新
		chromedp.ActionFunc(func(ctx context.Context) error {
			networkCookies, err := network.GetCookies().Do(ctx)
			if err != nil {
				return err
			}
			var cookieStrs []string
			for _, c := range networkCookies {
				cookieStrs = append(cookieStrs, fmt.Sprintf("%s=%s", c.Name, c.Value))
			}
			session.Cookies = strings.Join(cookieStrs, "; ")
			return nil
		}),
	)

	if err != nil {
		return "", err
	}

	if err := SaveSessionCache(CacheFile, session); err != nil {
		log.Printf("更新 Cookie 缓存失败: %v", err)
	}
	return orderNo, nil
}
