package scheduler

import (
	"fmt"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/sienchen/CorpRecon-Go/internal/auth"
	"github.com/sienchen/CorpRecon-Go/internal/exporter"
	"github.com/sienchen/CorpRecon-Go/internal/fetcher"
	"github.com/sienchen/CorpRecon-Go/internal/model"
	"github.com/sienchen/CorpRecon-Go/internal/parser"
)

// FetchConfig 抓取配置
type FetchConfig struct {
	MaxDepth           int
	WorkerCount        int
	EnableInvestments  bool
}

// Engine 核心调度引擎
type Engine struct {
	client       *fetcher.Client
	config       *FetchConfig
	visited      sync.Map // Key: EntID
	companyCache sync.Map // Key: EntID, Value: *model.CompanyEntity
}

// Request 代表一个抓取任务
type Request struct {
	EntName string // 公司名称 (阶段二和三都需要)
	EntID   string // 如果为空，表示需要先经过阶段二(搜索)
	Depth   int
}

// NewEngine 初始化引擎
func NewEngine(client *fetcher.Client, config *FetchConfig) *Engine {
	return &Engine{
		client: client,
		config: config,
	}
}

// Run 启动引擎开始抓取
func (e *Engine) Run(seedNames []string) {
	queue := make(chan Request, 1000)
	var wg sync.WaitGroup

	// 启动 Worker
	for i := 0; i < e.config.WorkerCount; i++ {
		go e.worker(i, queue, &wg)
	}

	// 放入种子
	for _, name := range seedNames {
		wg.Add(1)
		queue <- Request{EntName: name, EntID: "", Depth: 0}
	}

	wg.Wait() // 等待所有任务执行完毕
	close(queue)

	// 执行完毕后，导出详细的多个 CSV 文件
	prefix := "report"
	if len(seedNames) > 0 {
		prefix = seedNames[0]
	}
	timestamp := time.Now().Format("20060102_150405")
	prefix = fmt.Sprintf("%s_%s", prefix, timestamp)
	
	opts := exporter.ExportOptions{
		EnableInvestments:  e.config.EnableInvestments,
	}
	exporter.ExportMultipleCSV("output", prefix, &e.companyCache, opts)
}

func (e *Engine) worker(id int, queue chan Request, wg *sync.WaitGroup) {
	for req := range queue {
		e.process(id, req, queue, wg)
		wg.Done()
	}
}

func (e *Engine) process(id int, req Request, queue chan Request, wg *sync.WaitGroup) {
	if req.Depth > e.config.MaxDepth {
		return // 超过最大深度
	}

	entID := req.EntID
	entName := req.EntName

	// 阶段二 (Search)：如果缺少 EntID，先搜索补全
	if entID == "" {
		fmt.Printf("\n[*] (深度: %d) 启动阶段二(Search)查询: %s\n", req.Depth, entName)

		searchPayload := []byte(fmt.Sprintf(`{"queryType":"1","searchKey":"%s","pageNo":1,"range":10,"selectConditionData":"{\"status\":\"\",\"sort_field\":\"\",\"keyword_fields\":\"\"}"}`, entName))
		searchHeaders := map[string]string{
			"accept":       "application/json",
			"content-type": "application/json",
			"app-device":   "WEB",
		}

		searchRes, err := e.client.PostJSON("https://www.riskbird.com/riskbird-api/api/v1/companies/search", searchPayload, searchHeaders)
		if err != nil {
			log.Printf("Worker %d search request error: %v", id, err)
			return
		}

		companyInfo, err := parser.ParseSearch(searchRes)
		if err != nil {
			log.Printf("Worker %d search parse error for %s: %v", id, entName, err)
			return
		}

		entID = companyInfo.EntID
		// Search API 返回的高亮名称带有 <em> 和 </em> 标签，必须剔除，否则会导致 400 错误
		entName = strings.ReplaceAll(strings.ReplaceAll(companyInfo.EntName, "<em>", ""), "</em>", "")
		
		// 将基础信息放入缓存
		e.companyCache.Store(entID, &model.CompanyEntity{
			EntID:  entID,
			EntName: entName,
			Faren:   companyInfo.Faren,
			RegCap:  companyInfo.RegCap,
		})
		log.Printf("Worker %d resolved %s -> entid: %s", id, entName, entID)
	}

	// 检查去重缓存，基于唯一的 EntID
	if _, loaded := e.visited.LoadOrStore(entID, true); loaded {
		return // 已经抓取过
	}

	fmt.Printf("\n[*] (深度: %d) 启动阶段三(Investigate)抓取: %s (%s)\n", req.Depth, entName, entID)

	// 确保缓存中有实体
	var entity *model.CompanyEntity
	if val, ok := e.companyCache.Load(entID); ok {
		entity = val.(*model.CompanyEntity)
	} else {
		entity = &model.CompanyEntity{EntID: entID, EntName: entName}
		e.companyCache.Store(entID, entity)
	}
	
	// 获取公司的主 orderNo
	orderNo, err := e.fetchOrderNo(entName, entID)
	if err != nil {
		log.Printf("Worker %d failed to fetch master orderNo for %s: %v", id, entName, err)
		// 如果无法获取 orderNo，后续 API 必然全部失败（报 9999/500），直接跳过该公司的抓取
		return
	}

	// 阶段三 (Investigate)：抓取各维度数据
	var internalWg sync.WaitGroup

	if e.config.EnableInvestments {
		internalWg.Add(1)
		go e.fetchInvestments(id, req.Depth, entName, entID, orderNo, entity, queue, wg, &internalWg)
	}



	internalWg.Wait()
}



// fetchInvestments 抓取对外投资 (使用 graphics/query 接口)
func (e *Engine) fetchInvestments(id int, depth int, entName, entID string, orderNo string, entity *model.CompanyEntity, queue chan Request, globalWg *sync.WaitGroup, internalWg *sync.WaitGroup) {
	defer internalWg.Done()
	investHeaders := map[string]string{
		"accept":          "application/json",
		"content-type":    "application/json",
		"app-device":      "WEB",
		"origin":          "https://www.riskbird.com",
		// 复用 mother page 的 orderNo
		"referer":         fmt.Sprintf("https://www.riskbird.com/atlas/%s.html?entName=%s&orderNo=%s&dataType=entInvest", entID, url.QueryEscape(entName), orderNo),
	}

	investPayload := []byte(fmt.Sprintf(`{"entid":"%s","dataType":"entInvest","isExpand":0}`, entID))

	
	
	investRes, err := e.client.PostJSON("https://www.riskbird.com/riskbird-api/graphics/query", investPayload, investHeaders)
	if err != nil {
		log.Printf("Worker %d invest request error: %v", id, err)
		return
	}

	children, err := parser.ParseGraphicsInvestments(investRes)
	if err != nil {
		log.Printf("Worker %d invest parse error: %v", id, err)
		return
	}

	fmt.Printf("\n[+] (深度: %d) %s 查到 %d 家对外投资:\n", depth, entName, len(children))

	for _, child := range children {
		if child.EntName != "" && child.EntID != "" {
			fmt.Printf("    -> 发现对外投资: %s (ID: %s, 投资比例: %s, 认缴金额: %s)\n", child.EntName, child.EntID, child.FundedRatio, child.SubConAm)
			
			// 加入实体记录
			entity.Investments = append(entity.Investments, model.InvestmentItem{
				TargetEntID:   child.EntID,
				TargetEntName: child.EntName,
				SubConAm:      child.SubConAm,
				Date:          "", // graphics/query 不返回成立日期，暂时留空
			})

			// 获取/创建 child 的 entity (无论是否深入递归，都应被加入到总表中)
			var childEntity *model.CompanyEntity
			if val, ok := e.companyCache.Load(child.EntID); ok {
				childEntity = val.(*model.CompanyEntity)
			} else {
				childEntity = &model.CompanyEntity{EntID: child.EntID, EntName: child.EntName}
				e.companyCache.Store(child.EntID, childEntity)
			}

			// 针对 isLeaf == false 的节点，在允许的深度内仅对“对外投资”维度进行递归
			if !child.IsLeaf && depth+1 <= e.config.MaxDepth {
				globalWg.Add(1)
				go func(childName, childID string, childDepth int, cEntity *model.CompanyEntity) {
					defer globalWg.Done()
					
					var childInternalWg sync.WaitGroup
					
					childInternalWg.Add(1)
					// 仅调用 fetchInvestments，不放入全局 queue 以避免全维度抓取
					// 传入当前母页面的 orderNo 进行测试
					e.fetchInvestments(id, childDepth, childName, childID, orderNo, cEntity, queue, globalWg, &childInternalWg)

					
					childInternalWg.Wait()
				}(child.EntName, child.EntID, depth+1, childEntity)
			}
		}
	}
}

// fetchOrderNo 获取公司详情页HTML并正则匹配 orderNo
func (e *Engine) fetchOrderNo(entName, entID string) (string, error) {
	urlStr := fmt.Sprintf("https://www.riskbird.com/ent/%s.html?entid=%s", url.PathEscape(entName), entID)
	maxRetries := 6 // 给用户预留充足的时间手动滑动验证码

	for i := 0; i < maxRetries; i++ {
		req, err := http.NewRequest(http.MethodGet, urlStr, nil)
		if err != nil {
			return "", fmt.Errorf("failed to create request: %w", err)
		}
		req.Header.Set("app-device", "WEB")
		req.Header.Set("accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8")
		req.Header.Set("accept-language", "zh-CN,zh;q=0.9,en-US;q=0.8,en;q=0.7")

		htmlBytes, err := e.client.DoRequest(req)
		if err != nil {
			return "", fmt.Errorf("failed to get html: %w", err)
		}

		htmlStr := string(htmlBytes)
		re := regexp.MustCompile(`(WEB[0-9]{21})`)
		matches := re.FindStringSubmatch(htmlStr)
		if len(matches) > 1 {
			return matches[1], nil
		}

		if i < maxRetries-1 {
			log.Printf("\n[!] 触发防爬虫拦截！(无法在 %s 的页面中提取到 orderNo)", entName)
			log.Printf("    -> 准备自动唤起真实 Chromium 浏览器供您人工滑块排雷...")
			
			newOrderNo, solveErr := auth.SolveCaptchaInteractively(urlStr)
			if solveErr == nil && newOrderNo != "" {
				log.Printf("\n[+] 人工排雷成功！成功获取到新的 orderNo: %s", newOrderNo)
				return newOrderNo, nil
			}
			
			log.Printf("    -> 浏览器排雷超时或失败 (%v)，退回基础休眠重试 (剩余重试次数 %d)...休眠 10 秒后重试\n\n", solveErr, maxRetries-1-i)
			time.Sleep(10 * time.Second)
		} else {
			snippet := htmlStr
			if len(snippet) > 500 {
				snippet = snippet[:500]
			}
			return "", fmt.Errorf("orderNo not found in html (len: %d) after %d retries. Snippet: %s", len(htmlStr), maxRetries, snippet)
		}
	}
	return "", nil
}
