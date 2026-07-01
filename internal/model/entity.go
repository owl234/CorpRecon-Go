package model

// CompanyEntity 存储单个公司的多维全景聚合数据
type CompanyEntity struct {
	EntID      string `json:"entid"`       // 公司唯一标识
	EntName    string `json:"entName"`     // 公司名称
	Faren      string `json:"faren"`       // 法定代表人
	RegCap     string `json:"regCap"`      // 注册资本
	Status     string `json:"status"`      // 营业状态
	CompanyType string `json:"companyType"` // 企业类型

	// 以下字段为后续从各类 API 获取后追加
	Investments []InvestmentItem `json:"investments"` // 对外投资列表
}

// InvestmentItem 代表一条对外投资信息
type InvestmentItem struct {
	TargetEntID   string `json:"targetEntId"`
	TargetEntName string `json:"targetEntName"`
	SubConAm      string `json:"subConAm"` // 认缴/投资金额
	Date          string `json:"date"`     // 成立日期
}

