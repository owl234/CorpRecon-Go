package auth

import (
	"context"
	"fmt"
	"log"
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

// GetSession 启动无头浏览器，执行自动登录，并提取 Cookies 和 UA
func GetSession(ctx context.Context, cfg BrowserConfig) (*SessionData, error) {
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

	return &SessionData{
		Cookies:   cookieStr,
		UserAgent: userAgent,
	}, nil
}
