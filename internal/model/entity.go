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
	Trademarks  []TrademarkItem  `json:"trademarks"`  // 商标信息
	Apps        []AppItem        `json:"apps"`        // App/软件著作权
	ICPs        []ICPItem        `json:"icps"`        // 网络服务备案 (ICP)
	MiniProgs   []MiniProgItem   `json:"miniProgs"`   // 小程序
}

// InvestmentItem 代表一条对外投资信息
type InvestmentItem struct {
	TargetEntID   string `json:"targetEntId"`
	TargetEntName string `json:"targetEntName"`
	SubConAm      string `json:"subConAm"` // 认缴/投资金额
	Date          string `json:"date"`     // 成立日期
}

// TrademarkItem 代表一条商标信息
type TrademarkItem struct {
	Name     string `json:"name"`
	Category string `json:"category"`
	Status   string `json:"status"`
	IconHash string `json:"iconHash"` // 下载图片后计算得出的特征码
	Date     string `json:"date"`     // 申请日期/注册日期
}

// AppItem 代表一条 App/软著信息
type AppItem struct {
	Name    string `json:"name"`
	Version  string `json:"version"`
	IconHash string `json:"iconHash"`
	Date     string `json:"date"` // 更新日期/发布日期
}

// ICPItem 代表一条 ICP 备案信息
type ICPItem struct {
	Domain string `json:"domain"`
	Name   string `json:"name"`
	No     string `json:"no"` // 备案号
	Date   string `json:"date"` // 审核日期
}

// MiniProgItem 代表一条小程序信息
type MiniProgItem struct {
	Name     string `json:"name"`
	Category string `json:"category"`
	IconHash string `json:"iconHash"`
	Date     string `json:"date"` // 审核日期/更新日期
}
