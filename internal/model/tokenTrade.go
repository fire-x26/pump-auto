package model

import (
	"time"
)

// TradeDirection 交易方向枚举
type TradeDirection string

const (
	TRADE_DIRECTION_BUY  TradeDirection = "buy"  // 买入
	TRADE_DIRECTION_SELL TradeDirection = "sell" // 卖出
)

// TokenTrade 代表一笔代币交易
type TokenTrade struct {
	ID             uint64         `json:"id"`           // 交易ID
	TokenAddress   string         `json:"tokenAddress"` // 代币地址
	UserAddress    string         `json:"userAddress"`  // 用户钱包地址
	Amount         float64        `json:"amount"`       // 交易数量
	AmountUSD      float64        `json:"amountUsd"`    // 交易金额(USD)
	Price          float64        `json:"price"`        // 交易价格
	TradeDirection TradeDirection `json:"direction"`    // 交易方向(买/卖)
	TxHash         string         `json:"txHash"`       // 交易哈希
	BlockNumber    uint64         `json:"blockNumber"`  // 区块号
	Timestamp      time.Time      `json:"timestamp"`    // 交易时间(UTC)
}

// TokenHolders 代表代币持有者信息
type TokenHolders struct {
	TokenAddress     string    `json:"tokenAddress"`     // 代币地址
	TotalHolders     int       `json:"totalHolders"`     // 总持有者数量
	Top10Percentage  float64   `json:"top10Percentage"`  // Top10持有者占比
	Top50Percentage  float64   `json:"top50Percentage"`  // Top50持有者占比
	Top100Percentage float64   `json:"top100Percentage"` // Top100持有者占比
	UpdatedAt        time.Time `json:"updatedAt"`        // 数据更新时间
}
