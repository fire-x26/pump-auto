package execctor

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"pump_auto/internal/chainTx"
	"pump_auto/internal/common"
	"pump_auto/internal/ws"
	"sync"
	"time"
)

// MyPublicKey TODO: 请将此替换为您的机器人钱包的实际公钥!!!
var MyPublicKey string = "YOUR_BOT_WALLET_PUBLIC_KEY_HERE"

// 代币交易状态
type TokenTradeStatus int

const (
	StatusNone    TokenTradeStatus = iota
	StatusBought                   // 已买入
	StatusSelling                  // 卖出中
	StatusSold                     // 已卖出
)

const maxRecentPrices = 100 // 定义固定队列的长度
// WebSocket消息结构
type WSMessage struct {
	Method string   `json:"method"`
	Keys   []string `json:"keys,omitempty"`
	Data   struct {
		TokenAddress string  `json:"tokenAddress"`
		Price        float64 `json:"price"`
		Timestamp    int64   `json:"timestamp"`
	} `json:"data,omitempty"`
}

// 交易记录结构
type TradeRecord struct {
	Signature             string  `json:"signature"`
	Mint                  string  `json:"mint"` // 代币地址
	TraderPublicKey       string  `json:"traderPublicKey"`
	TxType                string  `json:"txType"` // buy/sell
	TokenAmount           float64 `json:"tokenAmount"`
	SolAmount             float64 `json:"solAmount"`
	NewTokenBalance       float64 `json:"newTokenBalance"`
	BondingCurveKey       string  `json:"bondingCurveKey"`
	VTokensInBondingCurve float64 `json:"vTokensInBondingCurve"`
	VSolInBondingCurve    float64 `json:"vSolInBondingCurve"`
	MarketCapSol          float64 `json:"marketCapSol"`
	Pool                  string  `json:"pool"`
}

// 价格跟踪信息
type PriceTrackInfo struct {
	Mint           string           // 代币符号
	EntryPrice     float64          // 买入价格 (下一步处理精度)
	HighestPrice   float64          // 历史最高价格 (基于原始价格流) (下一步处理精度)
	CurrentPrice   float64          // 当前最新原始价格 (下一步处理精度)
	BuyAmount      float64          // 买入数量 (最小单位)
	RemainingCoin  float64          // 剩余币量 (最小单位)
	SoldPercent    float64          // 已卖出百分比
	Status         TokenTradeStatus // 交易状态
	BuyTime        time.Time        // 买入时间
	LastUpdateTime time.Time        // 最新原始价格的更新时间

	mutex sync.Mutex // 保护并发访问
}

// 移动止盈设置
type TakeProfitSetting struct {
	PriceIncreasePct float64 // 价格涨幅百分比
	SellPct          float64 // 卖出百分比
}

// 移动止损设置
type StopLossSetting struct {
	ActivationPct float64 // 激活止损的涨幅百分比
	TriggerPct    float64 // 触发止损的跌幅百分比
	SellPct       float64 // 卖出百分比
}

// 交易执行器
type TradeExecutor struct {
	priceTracks map[string]*PriceTrackInfo // 价格跟踪，按代币地址索引
	mutex       sync.RWMutex               // 保护priceTracks的锁
	// stopLossRule    StopLossSetting            // 移动止损规则 - 当前策略下不直接使用其参数
	ctx             context.Context           // 上下文
	cancel          context.CancelFunc        // 取消函数
	triggeredLevels map[string]bool           // 已触发的止盈级别
	onTokenSold     func(tokenAddress string) // 新增字段：代币售出后的回调函数
}

// 创建新的交易执行器
func NewTradeExecutor(onTokenSoldCallback func(tokenAddress string)) *TradeExecutor {
	ctx, cancel := context.WithCancel(context.Background())

	return &TradeExecutor{
		priceTracks:     make(map[string]*PriceTrackInfo),
		ctx:             ctx,
		cancel:          cancel,
		triggeredLevels: make(map[string]bool),
		onTokenSold:     onTokenSoldCallback, // 保存回调函数
	}
}

func (t *TradeExecutor) ExpectBuyForToken(tokenAddress string, solToSpend float64, OutAmount float64) {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	if _, exists := t.priceTracks[tokenAddress]; exists {
		log.Printf("TradeExecutor: Token %s  is already being tracked or expected.", tokenAddress)
		return
	}

	t.priceTracks[tokenAddress] = &PriceTrackInfo{
		Mint:           tokenAddress,
		EntryPrice:     OutAmount / solToSpend,
		HighestPrice:   0,
		CurrentPrice:   0,
		BuyAmount:      OutAmount,
		RemainingCoin:  OutAmount,
		SoldPercent:    0,
		Status:         StatusBought,
		BuyTime:        time.Time{},
		LastUpdateTime: time.Time{},
		mutex:          sync.Mutex{},
	}
	log.Printf("TradeExecutor: Expecting our buy for token %s  Approx SOL to spend: %.4f. Waiting for our buy transaction message.",
		tokenAddress, solToSpend)
}

// UpdatePrice 处理原始价格流，更新直接受原始价格影响的字段
func (t *TradeExecutor) UpdatePrice(tokenAddress string, newRawPrice float64) {
	t.mutex.RLock()
	track, exists := t.priceTracks[tokenAddress]
	t.mutex.RUnlock()

	if !exists {
		return // 通常不应发生，因为此函数由特定代币的 aggregator 调用
	}

	track.mutex.Lock()
	defer track.mutex.Unlock()

	track.CurrentPrice = newRawPrice
	track.LastUpdateTime = time.Now()

	// 更新基于原始价格的历史最高价
	if newRawPrice > track.HighestPrice {
		track.HighestPrice = newRawPrice
		log.Printf("代币 %s 原始价格创新高: %.6f", track.Mint, newRawPrice)
	}
}

// 检查并执行策略(止盈/止损)
func (t *TradeExecutor) checkAndExecuteStrategies(tokenAddress string) {
	t.mutex.RLock()
	track, exists := t.priceTracks[tokenAddress]
	t.mutex.RUnlock()

	if !exists {
		return
	}

	track.mutex.Lock()
	defer track.mutex.Unlock()

	if track.RemainingCoin <= 0 {
		return
	}

	// 1. 新的兜底止损策略：如果当前最新原始价格低于买入价的5%
	if track.CurrentPrice < track.EntryPrice*0.95 {
		sellAmount := track.RemainingCoin
		lossPct := (track.EntryPrice - track.CurrentPrice) / track.EntryPrice * 100
		log.Printf("兜底止损触发: %s 当前价格 %.6f 低于买入价 %.6f 的 %.2f%%, 全部卖出 (约 %.6f 币)",
			track.Mint, track.CurrentPrice, track.EntryPrice, lossPct, sellAmount)
		t.executeTokenSellInternal(track, 1.00, tokenAddress, sellAmount, fmt.Sprintf("%.0f%%", 1.00*100), false, 20, 0.0005, common.PUMP) // Pass track

		return
	}

	// 2. 检查止盈条件 - 使用 track.CurrentPrice 以便测试中能精确控制策略检查所用的价格
	if t.checkTakeProfit(track, tokenAddress, track.CurrentPrice) {
		return
	}
}

// 检查并执行止盈策略 - 使用switch-case提高效率
// track 应已被外部锁定
func (t *TradeExecutor) checkTakeProfit(track *PriceTrackInfo, tokenAddress string, priceForStrategy float64) bool {
	currentPriceIncreasePct := (priceForStrategy - track.EntryPrice) / track.EntryPrice
	if track.EntryPrice == 0 { // Avoid division by zero
		currentPriceIncreasePct = 0
	}

	// DEBUG LOGGING
	log.Printf("DEBUG checkTakeProfit: token %s, priceForStrategy: %.15f, entryPrice: %.15f, currentPriceIncreasePct: %.15f",
		track.Mint, priceForStrategy, track.EntryPrice, currentPriceIncreasePct)

	levelKey := ""
	targetOverallSellPct := 0.0 // 目标总共卖出的原始购买量的百分比
	const epsilon = 1e-6        // 定义一个小的容差值

	switch {
	case currentPriceIncreasePct >= (1.00 - epsilon):
		levelKey = fmt.Sprintf("%s_100", tokenAddress)
		targetOverallSellPct = 1.00
	case currentPriceIncreasePct >= (0.90 - epsilon):
		levelKey = fmt.Sprintf("%s_90", tokenAddress)
		targetOverallSellPct = 0.90
	case currentPriceIncreasePct >= (0.80 - epsilon):
		levelKey = fmt.Sprintf("%s_80", tokenAddress)
		targetOverallSellPct = 0.80
	case currentPriceIncreasePct >= (0.70 - epsilon):
		levelKey = fmt.Sprintf("%s_70", tokenAddress)
		targetOverallSellPct = 0.70
	case currentPriceIncreasePct >= (0.60 - epsilon):
		levelKey = fmt.Sprintf("%s_60", tokenAddress)
		targetOverallSellPct = 0.60
	case currentPriceIncreasePct >= (0.50 - epsilon):
		levelKey = fmt.Sprintf("%s_50", tokenAddress)
		targetOverallSellPct = 0.50
	case currentPriceIncreasePct >= (0.40 - epsilon):
		levelKey = fmt.Sprintf("%s_40", tokenAddress)
		targetOverallSellPct = 0.40
	case currentPriceIncreasePct >= (0.30 - epsilon):
		levelKey = fmt.Sprintf("%s_30", tokenAddress)
		targetOverallSellPct = 0.30
	case currentPriceIncreasePct >= (0.20 - epsilon):
		levelKey = fmt.Sprintf("%s_20", tokenAddress)
		targetOverallSellPct = 0.20
	case currentPriceIncreasePct >= (0.10 - epsilon):
		levelKey = fmt.Sprintf("%s_10", tokenAddress)
		targetOverallSellPct = 0.10
	default:
		return false
	}

	if targetOverallSellPct < track.SoldPercent {
		log.Printf("当前level -- %s\n", levelKey)
		return true
	}
	sellAmount := track.BuyAmount * (targetOverallSellPct - track.SoldPercent)
	//t.executeTokenSellInternal(track, tokenAddress, sellAmount, fmt.Sprintf("%.0f%%", sellPctForThisLevel*100), false, 10, 0.0005, common.PUMP) // Pass track
	t.executeTokenSellInternal(track, targetOverallSellPct, tokenAddress, sellAmount, fmt.Sprintf("%.0f%%", targetOverallSellPct*100), false, 20, 0.0005, common.PUMP) // Pass track
	return true
}

// 此函数在调用时，track 应已被锁定
func (t *TradeExecutor) executeTokenSellInternal(track *PriceTrackInfo, SoldPercent float64, tokenAddress string, sellAmount float64, sellPercent string, denominatedInSol bool, slippage int, priorityFee float64, poolType common.PoolType) { // amount 类型改为 *big.Int
	if sellAmount <= 0 {                                                                                                                                                                                                                        // 避免卖出0或负数数量
		log.Printf("尝试卖出 %s 的数量 %s 过小或为0，取消卖出", tokenAddress, sellAmount)
		return
	}
	if sellPercent == "100%" {
		// 发送取消订阅消息
		if err := ws.UnsubscribeToTokenTrades([]string{tokenAddress}); err != nil {
			log.Printf("取消订阅代币 %s 失败: %v", tokenAddress, err)
		} else {
			log.Printf("成功取消订阅代币 %s 的交易事件", tokenAddress)
			// 调用回调函数通知Bot移除代币
			if t.onTokenSold != nil {
				t.onTokenSold(tokenAddress)
			}
		}
		return
	}
	_, err := chainTx.SellToken(tokenAddress, sellAmount, sellPercent, denominatedInSol, slippage, priorityFee, poolType)
	if err != nil {
		log.Printf("sell token  %s failed,error: %s", tokenAddress, err)
	}
	log.Printf("执行卖出: 代币 %s, 数量 (最小单位): %f ", tokenAddress, sellAmount)

	track.SoldPercent = SoldPercent
}

// 获取交易信息
func (t *TradeExecutor) GetTradeInfo(tokenAddress string) *PriceTrackInfo {
	t.mutex.RLock()
	defer t.mutex.RUnlock()

	if info, exists := t.priceTracks[tokenAddress]; exists {
		return info
	}
	return nil
}

// 停止交易执行器
func (t *TradeExecutor) Stop() {
	t.cancel()
	log.Println("交易执行器已停止")
}

// ProcessTradeMessage 处理从WebSocket收到的交易消息
func (t *TradeExecutor) ProcessTradeMessage(message []byte) {
	var tradeRecord TradeRecord
	if err := json.Unmarshal(message, &tradeRecord); err != nil {
		log.Printf("TradeExecutor: 解析WebSocket交易消息失败: %v, 原始消息: %s", err, string(message))
		return
	}
	log.Println("接收到交易消息:", tradeRecord)
	t.mutex.RLock() // Use RLock for checking existence first
	track, exists := t.priceTracks[tradeRecord.Mint]
	t.mutex.RUnlock()

	if !exists {
		// log.Printf("TradeExecutor: Received trade for token %s which we are not expecting. Ignoring.", tradeRecord.Mint)
		return // Token not expected at all
	}

	var price float64
	if tradeRecord.TokenAmount > 0 {
		price = tradeRecord.SolAmount / tradeRecord.TokenAmount
	} else {
		log.Printf("TradeExecutor: 警告: 交易记录 %s 中TokenAmount为0，无法计算价格: %v", tradeRecord.Mint, tradeRecord)
		return
	}

	// 只要代币在我们关注列表（不论状态是None, Bought, Selling），都更新其当前价格信息
	t.UpdatePrice(tradeRecord.Mint, price) // UpdatePrice 内部有自己的锁，会更新 CurrentPrice 和 HighestPrice

	// 只有在代币状态为 StatusBought 时才执行策略检查
	if track.Status != StatusSold { // 重新获取锁读取status是更安全的做法，或者确保UpdatePrice/checkAndExecuteStrategies正确处理并发
		t.checkAndExecuteStrategies(tradeRecord.Mint) // checkAndExecuteStrategies 内部应该有自己的锁来保护 track
	}

	// log.Printf("TradeExecutor: Processed trade for %s (%s) - Type: %s, Price: %.10f, TokenQty: %.6f",
	// 	tradeRecord.Mint, track.Symbol, tradeRecord.TxType, price, tradeRecord.TokenAmount)
}
