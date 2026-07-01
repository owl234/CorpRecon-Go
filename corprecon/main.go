package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/sienchen/CorpRecon-Go/internal/auth"
	"github.com/sienchen/CorpRecon-Go/internal/fetcher"
	"github.com/sienchen/CorpRecon-Go/internal/scheduler"
)

// Config 定义了通过文件注入的参数
type Config struct {
	Username    string   `json:"username"`
	Password    string   `json:"password"`
	Headless    bool     `json:"headless"`
	MaxDepth    int      `json:"max_depth"`
	WorkerCount int      `json:"worker_count"`
	Seeds       []string `json:"seeds"`
}


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

	// 0. 加载配置
	configPath := "config.json"
	var config Config

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		// 如果不存在，生成默认配置文件并退出
		defaultConfig := Config{
			Username:    "18905406473", // 默认占位
			Password:    "waifyy@0608", // 默认占位
			Headless:    false,
			MaxDepth:    3,
			WorkerCount: 1,
			Seeds:       []string{"江苏今日头条信息科技有限公司"},
		}
		data, _ := json.MarshalIndent(defaultConfig, "", "  ")
		if err := ioutil.WriteFile(configPath, data, 0644); err != nil {
			log.Fatalf("无法创建配置文件: %v", err)
		}
		log.Fatalf("\n[!] 检测到初次运行，已生成默认配置文件: %s\n[!] 请修改该文件（尤其是账号密码）后再次运行本程序。\n", configPath)
	}

	data, err := ioutil.ReadFile(configPath)
	if err != nil {
		log.Fatalf("读取配置文件失败: %v", err)
	}
	if err := json.Unmarshal(data, &config); err != nil {
		log.Fatalf("解析配置文件失败，请检查 JSON 格式: %v", err)
	}

	if len(config.Seeds) == 0 {
		log.Fatalf("配置文件中的 seeds 列表不能为空！")
	}

	// 1. 获取授权
	authCfg := auth.BrowserConfig{
		LoginURL: "https://www.riskbird.com",
		Username: config.Username,
		Password: config.Password,
		Timeout:  60 * time.Second,
		Headless: config.Headless,
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

	// 在这里可以自由组合您想爬取的维度开关！
	fetchConfig := &scheduler.FetchConfig{
		MaxDepth:           config.MaxDepth,
		WorkerCount:        config.WorkerCount,
		EnableInvestments:  true,
	}

	engine := scheduler.NewEngine(client, fetchConfig)

	// 4. 启动引擎，加入初始目标公司名称
	seeds := config.Seeds

	// 打印本次查询的配置信息
	fmt.Println("--------------------------------------------------")
	fmt.Println("[*] 当前任务配置信息：")
	fmt.Printf("    - 登录账号: %s\n", authCfg.Username)
	fmt.Printf("    - 无头模式 (Headless): %v\n", authCfg.Headless)
	fmt.Printf("    - 最大递归深度: %d\n", fetchConfig.MaxDepth)
	fmt.Printf("    - 并发工作协程: %d\n", fetchConfig.WorkerCount)
	fmt.Printf("    - 目标种子名单: %v\n", seeds)
	fmt.Println("--------------------------------------------------")

	// 阻塞启动
	engine.Run(seeds)

	log.Println("程序已跑完全部层级，并且数据已导出。")
	log.Println("程序正常退出。")
}
