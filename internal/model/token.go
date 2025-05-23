package model

// TokenMetadata 表示代币的元数据信息
type TokenMetadata struct {
	Name        string `json:"name"`
	Symbol      string `json:"symbol"`
	Description string `json:"description"`
	Image       string `json:"image"`
	ShowName    bool   `json:"showName"`
	CreatedOn   string `json:"createdOn"`
	Twitter     string `json:"twitter,omitempty"` // Twitter链接字段，可选
	Website     string `json:"website,omitempty"` // 网站链接字段，可选
}
