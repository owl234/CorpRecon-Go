package scheduler

import (
	"encoding/csv"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/sienchen/CorpRecon-Go/internal/fetcher"
	"github.com/sienchen/CorpRecon-Go/internal/model"
	"github.com/sienchen/CorpRecon-Go/internal/parser"
)

// Engine 核心调度引擎
type Engine struct {
	client       *fetcher.Client
	maxDepth     int
	workerCount  int
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
func NewEngine(client *fetcher.Client, maxDepth, workerCount int) *Engine {
	return &Engine{
		client:      client,
		maxDepth:    maxDepth,
		workerCount: workerCount,
	}
}

// Run 启动引擎开始抓取
func (e *Engine) Run(seedNames []string) {
	queue := make(chan Request, 1000)
	var wg sync.WaitGroup

	// 启动 Worker
	for i := 0; i < e.workerCount; i++ {
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
	e.ExportMultipleCSV("output", prefix)
}

func (e *Engine) worker(id int, queue chan Request, wg *sync.WaitGroup) {
	for req := range queue {
		e.process(id, req, queue, wg)
		wg.Done()
	}
}

func (e *Engine) process(id int, req Request, queue chan Request, wg *sync.WaitGroup) {
	if req.Depth > e.maxDepth {
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
		internalWg.Add(5)

		go e.fetchInvestments(id, req.Depth, entName, entID, orderNo, entity, queue, wg, &internalWg)
		go e.fetchTrademarks(id, req.Depth, entName, entID, orderNo, entity, &internalWg)
		go e.fetchApps(id, req.Depth, entName, entID, orderNo, entity, &internalWg)
		go e.fetchICPs(id, req.Depth, entName, entID, orderNo, entity, &internalWg)
		go e.fetchMiniProgs(id, req.Depth, entName, entID, orderNo, entity, &internalWg)

		internalWg.Wait()
}

// fetchICPs 抓取网络服务备案信息 (网站)
func (e *Engine) fetchICPs(id int, depth int, entName, entID string, orderNo string, entity *model.CompanyEntity, wg *sync.WaitGroup) {
	defer wg.Done()
	headers := map[string]string{
		"accept":          "application/json",
		"content-type":    "application/json",
		"xs-content-type": "application/json",
		"app-device":      "WEB",
		"referer":         fmt.Sprintf("https://www.riskbird.com/ent/%s.html?entid=%s", url.PathEscape(entName), entID),
	}


	page := 1
	pageSize := 10

	for {
		payload := []byte(fmt.Sprintf(`{"filterCnd":0,"page":%d,"size":%d,"orderNo":"%s","sortField":"","filterMap":{}}`, page, pageSize, orderNo))

		res, err := e.client.PostJSON("https://www.riskbird.com/riskbird-api/companyInfo/list/propertyIcp", payload, headers)
		if err != nil {
			log.Printf("Worker %d ICP request error: %v", id, err)
			break
		}

		totalCount, apiData, err := parser.ParseICPs(res)
		if err != nil {
			log.Printf("Worker %d ICP parse error: %v", id, err)
			break
		}

		fmt.Printf("\n[+] (深度: %d) %s [第 %d 页] 查到 %d 个ICP备案:\n", depth, entName, page, len(apiData))

		for _, icp := range apiData {
			if icp.Hostname != "" {
				fmt.Printf("    -> ICP备案: %s (域名: %s)\n", icp.Icpnum, icp.Hostname)
				entity.ICPs = append(entity.ICPs, model.ICPItem{
					Domain: icp.Hostname,
					Name:   icp.Webname, // 可能为空
					No:     icp.Icpnum,
					Date:   icp.ExamineDate,
				})
			}
		}

		if page*pageSize >= totalCount {
			break
		}
		page++
	}
}

// fetchMiniProgs 抓取微信小程序信息
func (e *Engine) fetchMiniProgs(id int, depth int, entName, entID string, orderNo string, entity *model.CompanyEntity, wg *sync.WaitGroup) {
	defer wg.Done()
	headers := map[string]string{
		"accept":          "application/json",
		"content-type":    "application/json",
		"xs-content-type": "application/json",
		"app-device":      "WEB",
		"referer":         fmt.Sprintf("https://www.riskbird.com/ent/%s.html?entid=%s", url.PathEscape(entName), entID),
	}


	page := 1
	pageSize := 10

	for {
		payload := []byte(fmt.Sprintf(`{"filterCnd":0,"page":%d,"size":%d,"orderNo":"%s","sortField":"","filterMap":{}}`, page, pageSize, orderNo))

		res, err := e.client.PostJSON("https://www.riskbird.com/riskbird-api/companyInfo/list/propertyIcpLittle", payload, headers)
		if err != nil {
			log.Printf("Worker %d MiniProg request error: %v", id, err)
			break
		}

		totalCount, apiData, err := parser.ParseMiniProgs(res)
		if err != nil {
			log.Printf("Worker %d MiniProg parse error: %v", id, err)
			break
		}

		fmt.Printf("\n[+] (深度: %d) %s [第 %d 页] 查到 %d 个小程序:\n", depth, entName, page, len(apiData))

		for _, mp := range apiData {
			if mp.Servicename != "" {
				fmt.Printf("    -> 小程序: %s (备案号: %s)\n", mp.Servicename, mp.IcpnumS)
				entity.MiniProgs = append(entity.MiniProgs, model.MiniProgItem{
					Name:     mp.Servicename,
					Category: "",
					IconHash: "",
					Date:     mp.UpdateDate,
				})
			}
		}

		if page*pageSize >= totalCount {
			break
		}
		page++
	}
}

// fetchApps 抓取知识产权-软件著作权信息
func (e *Engine) fetchApps(id int, depth int, entName, entID string, orderNo string, entity *model.CompanyEntity, wg *sync.WaitGroup) {
	defer wg.Done()
	headers := map[string]string{
		"accept":          "application/json",
		"content-type":    "application/json",
		"xs-content-type": "application/json",
		"app-device":      "WEB",
		"referer":         fmt.Sprintf("https://www.riskbird.com/ent/%s.html?entid=%s", url.PathEscape(entName), entID),
	}


	page := 1
	pageSize := 10

	for {
		// filterCnd is 0 here as per user curl payload
		appPayload := []byte(fmt.Sprintf(`{"filterCnd":0,"page":%d,"size":%d,"orderNo":"%s","sortField":"","filterMap":{}}`, page, pageSize, orderNo))

		appRes, err := e.client.PostJSON("https://www.riskbird.com/riskbird-api/companyInfo/list/propertyApp", appPayload, headers)
		if err != nil {
			log.Printf("Worker %d app request error: %v", id, err)
			break
		}

		totalCount, apiData, err := parser.ParseApps(appRes)
		if err != nil {
			log.Printf("Worker %d app parse error: %v", id, err)
			break
		}

		fmt.Printf("\n[+] (深度: %d) %s [第 %d 页] 查到 %d 个App:\n", depth, entName, page, len(apiData))

		for _, app := range apiData {
			if app.Appname != "" {
				// 获取 icon hash
				iconHash := ""
				if app.IconUrl != "" {
					hashStr, err := fetcher.GetIconHash(app.IconUrl)
					if err == nil {
						iconHash = hashStr
					} else {
						log.Printf("Worker %d failed to hash app icon %s: %v", id, app.IconUrl, err)
					}
				}

				fmt.Printf("    -> App: %s (Hash: %s)\n", app.Appname, iconHash)

				entity.Apps = append(entity.Apps, model.AppItem{
					Name:     app.Appname,
					Version:  "", // API 中没有提供版本信息
					IconHash: iconHash,
					Date:     app.UpdateDate,
				})
			}
		}

		if page*pageSize >= totalCount {
			break
		}
		page++
	}
}


// fetchTrademarks 抓取知识产权-商标信息并分页
func (e *Engine) fetchTrademarks(id int, depth int, entName, entID string, orderNo string, entity *model.CompanyEntity, wg *sync.WaitGroup) {
	defer wg.Done()
	tmHeaders := map[string]string{
		"accept":          "application/json",
		"content-type":    "application/json",
		"xs-content-type": "application/json",
		"app-device":      "WEB",
		"referer":         fmt.Sprintf("https://www.riskbird.com/ent/%s.html?entid=%s", url.PathEscape(entName), entID),
	}

	page := 1
	pageSize := 10

	for {
		tmPayload := []byte(fmt.Sprintf(`{"filterCnd":1,"page":%d,"size":%d,"orderNo":"%s","sortField":"","filterMap":{}}`, page, pageSize, orderNo))

		tmRes, err := e.client.PostJSON("https://www.riskbird.com/riskbird-api/companyInfo/list/propertyTm", tmPayload, tmHeaders)
		if err != nil {
			log.Printf("Worker %d trademark request error: %v", id, err)
			break
		}

		totalCount, apiData, err := parser.ParseTrademarks(tmRes)
		if err != nil {
			log.Printf("Worker %d trademark parse error: %v", id, err)
			break
		}

		fmt.Printf("\n[+] (深度: %d) %s [第 %d 页] 查到 %d 个商标:\n", depth, entName, page, len(apiData))

		for _, tm := range apiData {
			if tm.Tname != "" {
				// 获取 icon hash
				iconHash := ""
				if tm.PicPath != "" {
					hashStr, err := fetcher.GetIconHash(tm.PicPath)
					if err == nil {
						iconHash = hashStr
					} else {
						log.Printf("Worker %d failed to hash trademark icon %s: %v", id, tm.PicPath, err)
					}
				}

				fmt.Printf("    -> 商标: %s (分类: %s, 状态: %s, Hash: %s)\n", tm.Tname, tm.TmcatStr, tm.Pstatus, iconHash)

				entity.Trademarks = append(entity.Trademarks, model.TrademarkItem{
					Name:     tm.Tname,
					Category: tm.TmcatStr,
					Status:   tm.Pstatus,
					IconHash: iconHash,
					Date:     tm.AppDate,
				})
			}
		}

		if page*pageSize >= totalCount {
			break
		}
		page++
	}
}

// fetchInvestments 抓取对外投资并分页
func (e *Engine) fetchInvestments(id int, depth int, entName, entID string, orderNo string, entity *model.CompanyEntity, queue chan Request, globalWg *sync.WaitGroup, internalWg *sync.WaitGroup) {
	defer internalWg.Done()
	investHeaders := map[string]string{
		"accept":          "application/json",
		"content-type":    "application/json",
		"xs-content-type": "application/json",
		"app-device":      "WEB",
		"referer":         fmt.Sprintf("https://www.riskbird.com/ent/%s.html?entid=%s", url.PathEscape(entName), entID),
	}

	page := 1
	pageSize := 10

	for {
		investPayload := []byte(fmt.Sprintf(`{"filterCnd":1,"page":%d,"size":%d,"orderNo":"%s","sortField":"","filterMap":{"entstatus":"在营"}}`, page, pageSize, orderNo))

		investRes, err := e.client.PostJSON("https://www.riskbird.com/riskbird-api/companyInfo/list/companyInvest", investPayload, investHeaders)
		if err != nil {
			log.Printf("Worker %d invest request error: %v", id, err)
			break
		}

		totalCount, apiData, err := parser.ParseInvestments(investRes)
		if err != nil {
			log.Printf("Worker %d invest parse error: %v", id, err)
			break
		}

		fmt.Printf("\n[+] (深度: %d) %s [第 %d 页] 查到 %d 家对外投资:\n", depth, entName, page, len(apiData))

		for _, inv := range apiData {
			if inv.EntName != "" && inv.EntID != "" {
				fmt.Printf("    -> 发现对外投资: %s (ID: %s, 注册资本: %s)\n", inv.EntName, inv.EntID, inv.RegCap)
				
				// 加入实体记录
				entity.Investments = append(entity.Investments, model.InvestmentItem{
					TargetEntID:   inv.EntID,
					TargetEntName: inv.EntName,
					SubConAm:      inv.SubConAm,
					Date:          inv.Esdate,
				})

				// 阶段四：直接带着 entName 和 entID 投递，跳过 Stage 2
				globalWg.Add(1)
				go func(childName, childID string, childDepth int) {
					queue <- Request{EntName: childName, EntID: childID, Depth: childDepth}
				}(inv.EntName, inv.EntID, depth+1)
			}
		}

		if page*pageSize >= totalCount {
			break
		}
		page++
	}
}

// ExportMultipleCSV 将缓存的数据详细拆分导出为多个 CSV 文件
func (e *Engine) ExportMultipleCSV(dirName, prefix string) {
	if err := os.MkdirAll(dirName, 0755); err != nil {
		log.Printf("Failed to create output dir: %v", err)
		return
	}

	compFile, _ := os.Create(fmt.Sprintf("%s/%s_companies.csv", dirName, prefix))
	compWriter := csv.NewWriter(compFile)
	_ = compWriter.Write([]string{"公司ID", "公司名称", "法人", "注册资本", "对外投资数量", "商标数量", "App数量", "ICP备案数量", "小程序数量"})

	invFile, _ := os.Create(fmt.Sprintf("%s/%s_investments.csv", dirName, prefix))
	invWriter := csv.NewWriter(invFile)
	_ = invWriter.Write([]string{"主公司ID", "主公司名称", "投资目标ID", "投资目标名称", "认缴金额", "成立日期"})

	tmFile, _ := os.Create(fmt.Sprintf("%s/%s_trademarks.csv", dirName, prefix))
	tmWriter := csv.NewWriter(tmFile)
	_ = tmWriter.Write([]string{"公司ID", "公司名称", "商标名称", "分类", "状态", "图标Hash", "申请日期"})

	appFile, _ := os.Create(fmt.Sprintf("%s/%s_apps.csv", dirName, prefix))
	appWriter := csv.NewWriter(appFile)
	_ = appWriter.Write([]string{"公司ID", "公司名称", "App名称", "版本", "图标Hash", "更新日期"})

	icpFile, _ := os.Create(fmt.Sprintf("%s/%s_icps.csv", dirName, prefix))
	icpWriter := csv.NewWriter(icpFile)
	_ = icpWriter.Write([]string{"公司ID", "公司名称", "域名", "网站名称", "备案号", "审核日期"})

	mpFile, _ := os.Create(fmt.Sprintf("%s/%s_miniprogs.csv", dirName, prefix))
	mpWriter := csv.NewWriter(mpFile)
	_ = mpWriter.Write([]string{"公司ID", "公司名称", "小程序名称", "分类", "图标Hash", "更新日期"})

	e.companyCache.Range(func(key, value interface{}) bool {
		ent := value.(*model.CompanyEntity)
		
		_ = compWriter.Write([]string{
			ent.EntID, ent.EntName, ent.Faren, ent.RegCap,
			fmt.Sprintf("%d", len(ent.Investments)),
			fmt.Sprintf("%d", len(ent.Trademarks)),
			fmt.Sprintf("%d", len(ent.Apps)),
			fmt.Sprintf("%d", len(ent.ICPs)),
			fmt.Sprintf("%d", len(ent.MiniProgs)),
		})

		for _, inv := range ent.Investments {
			_ = invWriter.Write([]string{ent.EntID, ent.EntName, inv.TargetEntID, inv.TargetEntName, inv.SubConAm, inv.Date})
		}
		for _, tm := range ent.Trademarks {
			_ = tmWriter.Write([]string{ent.EntID, ent.EntName, tm.Name, tm.Category, tm.Status, tm.IconHash, tm.Date})
		}
		for _, app := range ent.Apps {
			_ = appWriter.Write([]string{ent.EntID, ent.EntName, app.Name, app.Version, app.IconHash, app.Date})
		}
		for _, icp := range ent.ICPs {
			_ = icpWriter.Write([]string{ent.EntID, ent.EntName, icp.Domain, icp.Name, icp.No, icp.Date})
		}
		for _, mp := range ent.MiniProgs {
			_ = mpWriter.Write([]string{ent.EntID, ent.EntName, mp.Name, mp.Category, mp.IconHash, mp.Date})
		}
		return true
	})

	compWriter.Flush(); compFile.Close()
	invWriter.Flush(); invFile.Close()
	tmWriter.Flush(); tmFile.Close()
	appWriter.Flush(); appFile.Close()
	icpWriter.Flush(); icpFile.Close()
	mpWriter.Flush(); mpFile.Close()

	log.Printf("\n[v] 详细数据导出完成: %s 目录下多个CSV文件\n", dirName)
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
			log.Printf("    -> 若您正在【有头模式】下运行，请立即在弹出的浏览器中手动滑动验证码！")
			log.Printf("    -> 正在等待人工介入 (剩余重试次数 %d)...休眠 10 秒后重试\n\n", maxRetries-1-i)
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
