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

// TrademarkResponse 代表商标信息接口的返回结构
type TrademarkResponse struct {
	Code int                   `json:"code"`
	Msg  string                `json:"msg"`
	Data TrademarkResponseData `json:"data"`
}

type TrademarkResponseData struct {
	TotalCount int               `json:"totalCount"`
	ApiData    []TrademarkApiData `json:"apiData"`
}

type TrademarkApiData struct {
	Tname   string `json:"tname"`
	TmcatStr string `json:"tmcatStr"`
	Pstatus string `json:"pstatus"`
	PicPath string `json:"picPath"`
	AppDate string `json:"appDate"` // 申请日期
}

// ParseTrademarks 解析商标接口的 JSON
func ParseTrademarks(jsonBody []byte) (int, []TrademarkApiData, error) {
	var resp TrademarkResponse
	if err := json.Unmarshal(jsonBody, &resp); err != nil {
		return 0, nil, fmt.Errorf("failed to unmarshal trademark response: %w", err)
	}

	if resp.Code != 20000 {
		return 0, nil, fmt.Errorf("trademark api returned error code: %d, msg: %s", resp.Code, resp.Msg)
	}

	return resp.Data.TotalCount, resp.Data.ApiData, nil
}

// AppResponse 代表App信息接口的返回结构
type AppResponse struct {
	Code int             `json:"code"`
	Msg  string          `json:"msg"`
	Data AppResponseData `json:"data"`
}

type AppResponseData struct {
	TotalCount int           `json:"totalCount"`
	ApiData    []AppApiData  `json:"apiData"`
}

type AppApiData struct {
	Appname string `json:"appname"`
	IconUrl string `json:"iconUrl"`
	Brief   string `json:"brief"`
	UpdateDate string `json:"updateDate"` // 更新日期/发布日期
}

// ParseApps 解析App接口的 JSON
func ParseApps(jsonBody []byte) (int, []AppApiData, error) {
	var resp AppResponse
	if err := json.Unmarshal(jsonBody, &resp); err != nil {
		return 0, nil, fmt.Errorf("failed to unmarshal app response: %w", err)
	}

	if resp.Code != 20000 {
		return 0, nil, fmt.Errorf("app api returned error code: %d, msg: %s", resp.Code, resp.Msg)
	}

	return resp.Data.TotalCount, resp.Data.ApiData, nil
}

// ICPResponse 代表网络服务备案接口的返回结构
type ICPResponse struct {
	Code int             `json:"code"`
	Msg  string          `json:"msg"`
	Data ICPResponseData `json:"data"`
}

type ICPResponseData struct {
	TotalCount int          `json:"totalCount"`
	ApiData    []ICPApiData `json:"apiData"`
}

type ICPApiData struct {
	Hostname string `json:"hostname"`
	Webname  string `json:"webname"`
	Icpnum   string `json:"icpnum"`
	ExamineDate string `json:"shdate"` // 审核通过日期
}

// ParseICPs 解析ICP接口的 JSON
func ParseICPs(jsonBody []byte) (int, []ICPApiData, error) {
	var resp ICPResponse
	if err := json.Unmarshal(jsonBody, &resp); err != nil {
		return 0, nil, fmt.Errorf("failed to unmarshal icp response: %w", err)
	}
	if resp.Code != 20000 {
		return 0, nil, fmt.Errorf("icp api returned error code: %d, msg: %s", resp.Code, resp.Msg)
	}
	return resp.Data.TotalCount, resp.Data.ApiData, nil
}

// MiniProgResponse 代表小程序接口的返回结构
type MiniProgResponse struct {
	Code int                  `json:"code"`
	Msg  string               `json:"msg"`
	Data MiniProgResponseData `json:"data"`
}

type MiniProgResponseData struct {
	TotalCount int               `json:"totalCount"`
	ApiData    []MiniProgApiData `json:"apiData"`
}

type MiniProgApiData struct {
	Servicename string `json:"servicename"`
	IcpnumS     string `json:"icpnumS"`
	UpdateDate  string `json:"updateDate"` // 更新日期
}

// ParseMiniProgs 解析小程序接口的 JSON
func ParseMiniProgs(jsonBody []byte) (int, []MiniProgApiData, error) {
	var resp MiniProgResponse
	if err := json.Unmarshal(jsonBody, &resp); err != nil {
		return 0, nil, fmt.Errorf("failed to unmarshal miniprog response: %w", err)
	}
	if resp.Code != 20000 {
		return 0, nil, fmt.Errorf("miniprog api returned error code: %d, msg: %s", resp.Code, resp.Msg)
	}
	return resp.Data.TotalCount, resp.Data.ApiData, nil
}
