package model

import "time"

// 消息类型
type MessageType int

const (
	MessageTypeBuy  MessageType = iota // 购买消息
	MessageTypeSell                    // 卖出消息
)

// 队列消息
type QueueMessage struct {
	Type         MessageType            // 消息类型
	TokenAddress string                 // 代币地址
	TokenSymbol  string                 // 代币符号
	TokenName    string                 // 代币名称
	TokenURI     string                 // 代币URI
	Price        float64                // 价格
	Amount       float64                // 数量
	Timestamp    time.Time              // 时间戳
	ExtraData    map[string]interface{} // 额外数据
}

// 创建购买消息
func NewBuyMessage(address string, symbol string, name string, uri string) *QueueMessage {
	return &QueueMessage{
		Type:         MessageTypeBuy,
		TokenAddress: address,
		TokenSymbol:  symbol,
		TokenName:    name,
		TokenURI:     uri,
		Timestamp:    time.Now(),
		ExtraData:    make(map[string]interface{}),
	}
}

// 创建卖出消息
func NewSellMessage(address string, symbol string, name string, price float64, amount float64) *QueueMessage {
	return &QueueMessage{
		Type:         MessageTypeSell,
		TokenAddress: address,
		TokenSymbol:  symbol,
		TokenName:    name,
		Price:        price,
		Amount:       amount,
		Timestamp:    time.Now(),
		ExtraData:    make(map[string]interface{}),
	}
}
