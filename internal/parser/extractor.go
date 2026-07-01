package parser

import (
	"encoding/json"
	"fmt"
)

// SearchResponse 代表风鸟网搜索接口的返回结构
type SearchResponse struct {
	Code int                `json:"code"`
	Msg  string             `json:"msg"`
	Data SearchResponseData `json:"data"`
}

type SearchResponseData struct {
	Total int                      `json:"total"`
	List  []SearchResponseDataList `json:"list"`
}

type SearchResponseDataList struct {
	EntName string `json:"entName"`
	EntID   string `json:"entid"`
	Faren   string `json:"faren"`
	RegCap  string `json:"regCap"`
}

// InvestResponse 代表对外投资接口的返回结构
type InvestResponse struct {
	Code int                `json:"code"`
	Msg  string             `json:"msg"`
	Data InvestResponseData `json:"data"`
}

type InvestResponseData struct {
	TotalCount int             `json:"totalCount"`
	ApiData    []InvestApiData `json:"apiData"`
}

type InvestApiData struct {
	EntID    string `json:"entid"`
	EntName  string `json:"entName"`
	RegCap   string `json:"regCap"`
	SubConAm string `json:"subConAm"` // 认缴金额/投资金额
	Esdate   string `json:"esDate"`   // 成立日期
}

// ParseSearch 解析搜索接口的 JSON，提取出目标公司的 EntID
func ParseSearch(jsonBody []byte) (*SearchResponseDataList, error) {
	var resp SearchResponse
	if err := json.Unmarshal(jsonBody, &resp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal search response: %w", err)
	}

	if resp.Code != 20000 {
		return nil, fmt.Errorf("search api returned error code: %d, msg: %s", resp.Code, resp.Msg)
	}

	if len(resp.Data.List) == 0 {
		return nil, fmt.Errorf("no company found in search results")
	}

	// 默认返回第一条最匹配的记录
	return &resp.Data.List[0], nil
}

// ParseInvestments 解析对外投资接口的 JSON，提取总数和列表
func ParseInvestments(jsonBody []byte) (int, []InvestApiData, error) {
	var resp InvestResponse
	if err := json.Unmarshal(jsonBody, &resp); err != nil {
		return 0, nil, fmt.Errorf("failed to unmarshal invest response: %w", err)
	}

	if resp.Code != 20000 {
		return 0, nil, fmt.Errorf("invest api returned error code: %d, msg: %s", resp.Code, resp.Msg)
	}

	return resp.Data.TotalCount, resp.Data.ApiData, nil
}



// GraphicsQueryResponse 代表新版对外投资图谱接口的返回结构
type GraphicsQueryResponse struct {
	Code    int               `json:"code"`
	Msg     string            `json:"msg"`
	Data    GraphicsQueryData `json:"data"`
	Success bool              `json:"success"`
}

type GraphicsQueryData struct {
	EntName  string               `json:"entname"`
	EntStatus string              `json:"entStatus"`
	RiskLevel string              `json:"riskLevel"`
	EntID    string               `json:"entid"`
	Children []GraphicsQueryChild `json:"children"`
}

type GraphicsQueryChild struct {
	EntID         string `json:"entid"`
	InvType       string `json:"invType"`
	EntName       string `json:"entname"`
	FundedRatio   string `json:"fundedRatio"`
	SubConAm      string `json:"subConAm"`
	EntStatus     string `json:"entStatus"`
	RiskLevel     string `json:"riskLevel"`
	IsLeaf        bool   `json:"isLeaf"`
	RelationEntID string `json:"relationEntid"`
	InvestorID    string `json:"investorId"`
}

// ParseGraphicsInvestments 解析图谱查询接口的 JSON
func ParseGraphicsInvestments(jsonBody []byte) ([]GraphicsQueryChild, error) {
	var resp GraphicsQueryResponse
	if err := json.Unmarshal(jsonBody, &resp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal graphics query response: %w", err)
	}
	if resp.Code != 20000 {
		return nil, fmt.Errorf("graphics query api returned error code: %d, msg: %s", resp.Code, resp.Msg)
	}
	return resp.Data.Children, nil
}
