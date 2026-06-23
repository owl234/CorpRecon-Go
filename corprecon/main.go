package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/sienchen/CorpRecon-Go/internal/auth"
	"github.com/sienchen/CorpRecon-Go/internal/fetcher"
	"github.com/sienchen/CorpRecon-Go/internal/scheduler"
)

func main() {
	banner := `
   _____                 _____                     
  / ____|               |  __ \                    
 | |     ___  _ __ _ __ | |__) |___  ___ ___  _ __ 
 | |    / _ \| '__| '_ \|  _  // _ \/ __/ _ \| '_ \
 | |___| (_) | |  | |_) | | \ \  __/ (_| (_) | | | |
  \_____\___/|_|  | .__/|_|  \_\___|\___\___/|_| |_|
                  | |    --- by CorpRecon-Go ---                    
                  |_|                              
`
	fmt.Println(banner)
	log.Println("=== CorpRecon-Go 混合模式爬虫启动 ===")

	// 1. 获取授权 (需通过环境变量或配置文件注入，此处以占位符为例)
	authCfg := auth.BrowserConfig{
		LoginURL: "https://www.riskbird.com",
		Username: "YOUR_PHONE_NUMBER",        // 请填入您的真实账号
		Password: "YOUR_PASSWORD",            // 请填入您的真实密码
		Timeout:  60 * time.Second,
		Headless: false, // 强烈建议测试时设为 false，以防出现验证码需要手动滑
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 处理优雅退出
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		log.Println("收到退出信号，准备清理...")
		cancel()
	}()

	log.Println("[1/3] 正在唤醒无头浏览器获取授权...")
	session, err := auth.GetSession(ctx, authCfg)
	if err != nil {
		log.Fatalf("获取授权失败: %v\n请检查配置或网站是否引入了人机验证（验证码）。", err)
	}
	log.Printf("授权成功！获取到 Cookie 长度: %d, UA: %s", len(session.Cookies), session.UserAgent)

	// 2. 初始化 Fetcher 模块
	log.Println("[2/3] 初始化高速并发抓取客户端...")
	client := fetcher.NewClient(session)

	// 3. 初始化 Scheduler 模块
	log.Println("[3/3] 启动调度引擎...")
	maxDepth := 1
	workerCount := 2
	engine := scheduler.NewEngine(client, maxDepth, workerCount) // 最大深度改为1，为了让测试能快速跑完出结果

	// 3. 启动引擎，加入初始目标公司名称
	// 我们选用一个体量较小的公司作为测试种子，以便几分钟内能跑出完整的 CSV 报告
	seeds := []string{
		"竞技世界（成都）网络技术有限公司",
	}

	// 打印本次查询的配置信息
	fmt.Println("--------------------------------------------------")
	fmt.Println("[*] 当前任务配置信息：")
	fmt.Printf("    - 登录账号: %s\n", authCfg.Username)
	fmt.Printf("    - 无头模式 (Headless): %v\n", authCfg.Headless)
	fmt.Printf("    - 最大递归深度: %d\n", maxDepth)
	fmt.Printf("    - 并发工作协程: %d\n", workerCount)
	fmt.Printf("    - 目标种子名单: %v\n", seeds)
	fmt.Println("--------------------------------------------------")

	// 阻塞启动
	engine.Run(seeds)

	log.Println("程序已跑完全部层级，并且数据已导出。")
	log.Println("程序正常退出。")
}
