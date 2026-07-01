package exporter

import (
	"encoding/csv"
	"fmt"
	"log"
	"os"
	"sync"

	"github.com/sienchen/CorpRecon-Go/internal/model"
)

// ExportOptions 控制要导出的 CSV 文件
type ExportOptions struct {
	EnableInvestments  bool
}

// ExportMultipleCSV 将缓存的数据详细拆分导出为多个 CSV 文件
func ExportMultipleCSV(dirName, prefix string, companyCache *sync.Map, opts ExportOptions) {
	if err := os.MkdirAll(dirName, 0755); err != nil {
		log.Printf("Failed to create output dir: %v", err)
		return
	}

	namesFile, _ := os.Create(fmt.Sprintf("%s/%s_company_names.txt", dirName, prefix))
	defer namesFile.Close()

	compFile, _ := os.Create(fmt.Sprintf("%s/%s_companies.csv", dirName, prefix))
	compWriter := csv.NewWriter(compFile)

	compHeaders := []string{"公司ID", "公司名称", "法人", "注册资本"}
	if opts.EnableInvestments { compHeaders = append(compHeaders, "对外投资数量") }
	_ = compWriter.Write(compHeaders)

	var invFile *os.File
	var invWriter *csv.Writer

	if opts.EnableInvestments {
		invFile, _ = os.Create(fmt.Sprintf("%s/%s_investments.csv", dirName, prefix))
		invWriter = csv.NewWriter(invFile)
		_ = invWriter.Write([]string{"主公司ID", "主公司名称", "投资目标ID", "投资目标名称", "认缴金额", "成立日期"})
	}



	companyCache.Range(func(key, value interface{}) bool {
		ent := value.(*model.CompanyEntity)
		
		_, _ = namesFile.WriteString(ent.EntName + "\n")
		
		compRow := []string{ent.EntID, ent.EntName, ent.Faren, ent.RegCap}
		if opts.EnableInvestments { compRow = append(compRow, fmt.Sprintf("%d", len(ent.Investments))) }
		_ = compWriter.Write(compRow)

		if opts.EnableInvestments && invWriter != nil {
			for _, inv := range ent.Investments {
				_ = invWriter.Write([]string{ent.EntID, ent.EntName, inv.TargetEntID, inv.TargetEntName, inv.SubConAm, inv.Date})
			}
		}

		return true
	})

	compWriter.Flush(); compFile.Close()
	if opts.EnableInvestments && invWriter != nil { invWriter.Flush(); invFile.Close() }

	log.Printf("\n[v] 详细数据导出完成: %s 目录下多个CSV文件以及纯文本公司名单\n", dirName)
}
