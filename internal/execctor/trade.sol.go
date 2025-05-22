package execctor

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"sync"
	"time"
)

// 代币交易状态
type TokenTradeStatus int

const (
	StatusNone    TokenTradeStatus = iota
	StatusBought                   // 已买入
	StatusSelling                  // 卖出中
	StatusSold                     // 已卖出
)

const maxRecentPrices = 100 // 定义固定队列的长度

// 价格跟踪信息
type PriceTrackInfo struct {
	Symbol         string           // 代币符号
	EntryPrice     float64          // 买入价格 (下一步处理精度)
	HighestPrice   float64          // 历史最高价格 (基于原始价格流) (下一步处理精度)
	CurrentPrice   float64          // 当前最新原始价格 (下一步处理精度)
	BuyAmount      *big.Int         // 买入数量 (最小单位)
	RemainingCoin  *big.Int         // 剩余币量 (最小单位)
	RemainPercent  float64          // 剩余百分比
	Status         TokenTradeStatus // 交易状态
	BuyTime        time.Time        // 买入时间
	LastUpdateTime time.Time        // 最新原始价格的更新时间
	Decimals       uint8            // 代币的精度

	// RecentPrices []struct { // 记录最近的原始价格数据，用于短期跌幅监控
	// 	Price     float64
	// 	Timestamp time.Time
	// }

	// 聚合价格数据
	// LatestPrice15s float64 // 最近15秒聚合价格 (取间隔末尾的原始价)
	// LatestPrice30s float64 // 最近30秒聚合价格
	// LatestPrice1m  float64 // 最近1分钟聚合价格
	// LatestPrice5m  float64 // 最近5分钟聚合价格

	// // K线周期对齐的聚合时间戳 (记录的是周期的开始时间)
	// LastAggregatedCycleStart15s time.Time
	// LastAggregatedCycleStart30s time.Time
	// LastAggregatedCycleStart1m  time.Time
	// LastAggregatedCycleStart5m  time.Time

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
	// ticker        *time.Ticker // 跌幅15%
}

// 交易执行器
type TradeExecutor struct {
	priceTracks map[string]*PriceTrackInfo // 价格跟踪，按代币地址索引
	mutex       sync.RWMutex               // 保护priceTracks的锁
	// stopLossRule    StopLossSetting            // 移动止损规则 - 当前策略下不直接使用其参数
	ctx             context.Context    // 上下文
	cancel          context.CancelFunc // 取消函数
	triggeredLevels map[string]bool    // 已触发的止盈级别
}

// 创建新的交易执行器
func NewTradeExecutor() *TradeExecutor {
	ctx, cancel := context.WithCancel(context.Background())

	return &TradeExecutor{
		priceTracks:     make(map[string]*PriceTrackInfo),
		ctx:             ctx,
		cancel:          cancel,
		triggeredLevels: make(map[string]bool),
	}
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
		log.Printf("代币 %s 原始价格创新高: %.6f", track.Symbol, newRawPrice)
	}
}

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

// trackPriceStrategy 定期检查并执行交易策略 (基于聚合价格)
func (t *TradeExecutor) trackPriceStrategy(tokenAddress string) {
	strategyCheckTicker := time.NewTicker(10 * time.Second) // 每10秒检查一次策略
	defer strategyCheckTicker.Stop()

	for {
		select {
		case <-t.ctx.Done():
			log.Printf("策略检查器 %s 停止", tokenAddress)
			return
		case <-strategyCheckTicker.C:

			t.checkAndExecuteStrategies(tokenAddress)
		}
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

	if track.Status != StatusBought {
		return // 不是已买入状态（可能正在卖出或已卖出）
	}

	if track.RemainingCoin.Cmp(big.NewInt(0)) <= 0 {
		return
	}

	// 0. 紧急止损检查 (基于固定数量的 RecentPrices 队列的头尾价格)
	// triggeredStop, dropReason, actualDropPct := t.checkFixedQueueFallStop(track, 0.05)
	// if triggeredStop {
	// 	sellAmount := track.RemainingCoin
	// 	log.Printf("固定队列止损触发 (%s): %s 价格跌幅 %.2f%%, 全部卖出 (约 %.6f 币)",
	// 		dropReason, track.Symbol, actualDropPct*100, sellAmount)
	// 	t.executeTokenSellInternal(track, tokenAddress, sellAmount)
	// 	return
	// }

	// 1. 新的兜底止损策略：如果当前最新原始价格低于买入价的5%
	if track.CurrentPrice < track.EntryPrice*0.95 {
		sellAmount := track.RemainingCoin
		lossPct := (track.EntryPrice - track.CurrentPrice) / track.EntryPrice * 100
		log.Printf("兜底止损触发: %s 当前价格 %.6f 低于买入价 %.6f 的 %.2f%%, 全部卖出 (约 %.6f 币)",
			track.Symbol, track.CurrentPrice, track.EntryPrice, lossPct, sellAmount)
		t.executeTokenSellInternal(track, tokenAddress, sellAmount, 1.00)
		return
	}

	// 2. 检查止盈条件 - 使用 track.CurrentPrice 以便测试中能精确控制策略检查所用的价格
	if t.checkTakeProfit(track, tokenAddress, track.CurrentPrice) {
		return
	}
}

// 检查固定长度队列的头尾价格跌幅
// func (t *TradeExecutor) checkFixedQueueFallStop(track *PriceTrackInfo, fallPercent float64) (triggered bool, reason string, actualFallPct float64) {
// 	queueLen := len(track.RecentPrices)
// 	if queueLen < maxRecentPrices { // 队列未满，不执行此止损，或根据需求调整逻辑
// 		return false, "队列未满", 0
// 	}

// 	headPrice := track.RecentPrices[0].Price          // 队列中最早的价格
// 	tailPrice := track.RecentPrices[queueLen-1].Price // 队列中最新的价格 (等同于 track.CurrentPrice)

// 	if headPrice > 0 && tailPrice < headPrice { // 必须有实际的下跌，且最早价格为正
// 		dropPct := (headPrice - tailPrice) / headPrice
// 		if dropPct >= fallPercent { // 跌幅达到指定百分比
// 			return true, fmt.Sprintf("最近%d个价格点内跌幅%.2f%%", maxRecentPrices, fallPercent*100), dropPct
// 		}
// 	}
// 	return false, "", 0
// }

// 检查并执行止盈策略 - 使用switch-case提高效率
// track 应已被外部锁定
func (t *TradeExecutor) checkTakeProfit(track *PriceTrackInfo, tokenAddress string, priceForStrategy float64) bool {
	currentPriceIncreasePct := (priceForStrategy - track.EntryPrice) / track.EntryPrice
	if track.EntryPrice == 0 { // Avoid division by zero
		currentPriceIncreasePct = 0
	}

	// DEBUG LOGGING
	log.Printf("DEBUG checkTakeProfit: token %s, priceForStrategy: %.15f, entryPrice: %.15f, currentPriceIncreasePct: %.15f",
		track.Symbol, priceForStrategy, track.EntryPrice, currentPriceIncreasePct)

	levelKey := ""
	targetOverallSellPct := 0.0 // 目标总共卖出的原始购买量的百分比
	const epsilon = 1e-9        // 定义一个小的容差值

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

	// 检查该级别是否已触发过以此百分比卖出
	// 注意：这里的 triggeredLevels 是全局的，需要 TradeExecutor 的锁
	t.mutex.Lock() // Lock for t.triggeredLevels
	triggered, exists := t.triggeredLevels[levelKey]
	if exists && triggered {
		t.mutex.Unlock()
		return false
	}
	t.triggeredLevels[levelKey] = true
	t.mutex.Unlock()

	// 不计算具体数量，直接使用百分比
	sellPctForThisLevel := targetOverallSellPct // 直接使用目标百分比

	// 创建一个big.Float表示sellPctForThisLevel
	sellPctBigFloat := new(big.Float).SetFloat64(sellPctForThisLevel)
	// 将BuyAmount转为big.Float
	buyAmountBigFloat := new(big.Float).SetInt(track.BuyAmount)
	// 计算卖出数量
	resultBigFloat := new(big.Float).Mul(buyAmountBigFloat, sellPctBigFloat)
	// 转回big.Int
	sellAmount := new(big.Int)
	resultBigFloat.Int(sellAmount)

	log.Printf("止盈触发: %s 使用策略价格 %.6f (盈利 %.2f%%), 达到级别 %.2f%%. 目标总卖出 %.2f%%, 本次卖出初始购买量的 %.2f%% (%s 最小单位)",
		track.Symbol, priceForStrategy, currentPriceIncreasePct*100, targetOverallSellPct*100, targetOverallSellPct*100, sellPctForThisLevel*100, sellAmount.String())

	t.executeTokenSellInternal(track, tokenAddress, sellAmount, sellPctForThisLevel) // Pass track
	return true
}

// 此函数在调用时，track 应已被锁定
func (t *TradeExecutor) executeTokenSellInternal(track *PriceTrackInfo, tokenAddress string, amount *big.Int, sellPct float64) { // amount 类型改为 *big.Int
	if amount.Cmp(big.NewInt(0)) <= 0 || sellPct <= 0 { // 避免卖出0或负数数量
		log.Printf("尝试卖出 %s 的数量 %s 过小或为0，取消卖出", track.Symbol, amount.String())
		return
	}

	log.Printf("模拟执行卖出: 代币 %s, 数量 (最小单位): %s, 当前策略价格: %.6f, 当前原始价格: %.6f", track.Symbol, amount.String(), track.CurrentPrice)

	track.RemainingCoin.Sub(track.RemainingCoin, amount)
	track.RemainPercent = float64(track.RemainingCoin.Int64()) / float64(track.BuyAmount.Int64())
	if track.RemainingCoin.Cmp(big.NewInt(0)) <= 0 { // 如果剩余币量小于等于0
		track.RemainingCoin.SetInt64(0) // 确保为0，避免负数
		track.Status = StatusSold
		log.Printf("代币 %s 已全部卖出", track.Symbol)
	}
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

// 添加测试价格驱动函数
func (t *TradeExecutor) SimulatePriceChange(tokenAddress string, priceMultiplier float64) {
	t.mutex.RLock()
	track, exists := t.priceTracks[tokenAddress]
	t.mutex.RUnlock()

	if !exists {
		log.Printf("代币 %s 不存在，无法模拟价格变化", tokenAddress)
		return
	}

	var oldPrice float64
	var entryPrice float64

	track.mutex.Lock() // Use Lock for sync.Mutex
	oldPrice = track.CurrentPrice
	entryPrice = track.EntryPrice
	track.mutex.Unlock() // Use Unlock for sync.Mutex

	newPrice := entryPrice * priceMultiplier

	log.Printf("模拟代币 %s 价格变化: %.6f -> %.6f (涨幅: %.2f%%)",
		tokenAddress, oldPrice, newPrice, (priceMultiplier-1.0)*100)

	// Update the primary price record. This sets track.CurrentPrice = newPrice.
	t.UpdatePrice(tokenAddress, newPrice)

	// For the test to work as intended, the strategy check needs to use this newPrice.
	// Force update LatestPrice1m which is used by the strategy check.
	// This ensures that the checkAndExecuteStrategies call below uses the explicitly simulated price.

	// Now execute strategies with LatestPrice1m having been set to newPrice
	t.checkAndExecuteStrategies(tokenAddress)
}

// 运行一个价格模拟测试
func (t *TradeExecutor) RunPriceSimulationTest(tokenAddress string) {
	log.Printf("开始对代币 %s 进行价格模拟测试", tokenAddress)

	// 模拟价格涨跌的场景
	scenarios := []struct {
		Description     string
		PriceMultiplier float64
		SleepSeconds    int
	}{
		{"初始上涨", 1.05, 1},
		{"持续上涨", 1.10, 1},
		{"达到第一个止盈点", 1.15, 2},
		{"进一步上涨", 1.25, 1},
		{"达到第二个止盈点", 1.30, 2},
		{"快速上涨", 1.50, 2},
		{"更快上涨", 1.70, 1},
		{"剧烈上涨", 1.90, 1},
		{"到达顶点", 2.20, 2},
		{"开始下跌", 2.10, 1},
		{"继续下跌", 1.95, 1},
		{"继续下跌", 1.85, 1},
		{"接近止损点", 1.70, 1},
		{"触发止损", 1.55, 2}, // 应该触发止损，卖出所有
	}

	for _, s := range scenarios {
		log.Printf("模拟场景: %s", s.Description)
		t.SimulatePriceChange(tokenAddress, s.PriceMultiplier)
		time.Sleep(time.Duration(s.SleepSeconds) * time.Second)
	}

	log.Printf("代币 %s 价格模拟测试完成", tokenAddress)
}

// 输出代币持仓状态
func (t *TradeExecutor) PrintTradeStatus(tokenAddress string) {
	t.mutex.RLock()
	track, exists := t.priceTracks[tokenAddress]
	t.mutex.RUnlock()

	if !exists {
		log.Printf("代币 %s 不在跟踪列表中", tokenAddress)
		return
	}

	track.mutex.Lock()
	defer track.mutex.Unlock()

	log.Printf(`
		代币: %s
		买入价格: %.6f
		当前价格: %.6f
		历史最高价: %.6f
		买入数量: %s
		剩余数量: %s
		当前收益: %.6f
		交易状态: %v
		--------------------------
		`,
		track.Symbol,
		track.EntryPrice,
		track.CurrentPrice,
		track.HighestPrice,
		track.BuyAmount.String(),
		track.RemainingCoin.String(),
		((track.CurrentPrice/track.EntryPrice)-1)*100,
		track.Status)
}

// ProcessTradeMessage 处理从WebSocket收到的交易消息
func (t *TradeExecutor) ProcessTradeMessage(message []byte) {
	// 解析交易记录
	var tradeRecord TradeRecord
	if err := json.Unmarshal(message, &tradeRecord); err != nil {
		log.Printf("解析WebSocket交易消息失败: %v, 原始消息: %s", err, string(message))
		return
	}

	// 检查是否为我们跟踪的代币
	t.mutex.RLock()
	_, exists := t.priceTracks[tradeRecord.Mint]
	t.mutex.RUnlock()

	if !exists {
		// 不是我们跟踪的代币，忽略
		return
	}

	// 计算价格 (SOL/代币单价)
	var price float64
	if tradeRecord.TokenAmount > 0 {
		price = tradeRecord.SolAmount / tradeRecord.TokenAmount
	} else {
		log.Printf("警告: 交易记录中TokenAmount为0，无法计算价格: %v", tradeRecord)
		return
	}

	// 更新价格
	t.UpdatePrice(tradeRecord.Mint, price)
	t.checkAndExecuteStrategies(tradeRecord.Mint)
	log.Printf("收到 %s 的新交易(%s): 价格 %.10f SOL/代币, 代币数量: %.6f, SOL: %.6f",
		tradeRecord.Mint, tradeRecord.TxType, price, tradeRecord.TokenAmount, tradeRecord.SolAmount)
}
