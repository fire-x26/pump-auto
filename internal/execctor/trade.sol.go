package execctor

import (
	"context"
	"fmt"
	"log"
	"math"
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
	Status         TokenTradeStatus // 交易状态
	BuyTime        time.Time        // 买入时间
	LastUpdateTime time.Time        // 最新原始价格的更新时间
	Decimals       uint8            // 代币的精度

	RecentPrices []struct { // 记录最近的原始价格数据，用于短期跌幅监控
		Price     float64
		Timestamp time.Time
	}

	// 聚合价格数据
	LatestPrice15s float64 // 最近15秒聚合价格 (取间隔末尾的原始价)
	LatestPrice30s float64 // 最近30秒聚合价格
	LatestPrice1m  float64 // 最近1分钟聚合价格
	LatestPrice5m  float64 // 最近5分钟聚合价格

	// K线周期对齐的聚合时间戳 (记录的是周期的开始时间)
	LastAggregatedCycleStart15s time.Time
	LastAggregatedCycleStart30s time.Time
	LastAggregatedCycleStart1m  time.Time
	LastAggregatedCycleStart5m  time.Time

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

// 买入代币
func (t *TradeExecutor) BuyToken(tokenAddress string, tokenSymbol string, amount float64, price float64, decimals uint8) error {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	if _, exists := t.priceTracks[tokenAddress]; exists {
		return fmt.Errorf("已经买入此代币: %s", tokenAddress)
	}

	// 将 float64 amount 转换为 *big.Int 最小单位
	amountInt := new(big.Int)
	if amount > 0 {
		amountBigFloat := new(big.Float).SetFloat64(amount)
		scaleFactor := new(big.Float).SetUint64(uint64(math.Pow10(int(decimals))))
		amountScaled := new(big.Float).Mul(amountBigFloat, scaleFactor)
		amountScaled.Int(amountInt) // 转换并截断小数部分（通常代币量不应有小数的最小单位）
	}

	log.Printf("买入代币 %s (%s), 数量 (float): %.6f, 价格: %.6f, Decimals: %d, 数量 (big.Int): %s", tokenAddress, tokenSymbol, amount, price, decimals, amountInt.String())

	now := time.Now()
	t.priceTracks[tokenAddress] = &PriceTrackInfo{
		Symbol:         tokenSymbol,
		EntryPrice:     price,
		HighestPrice:   price,
		CurrentPrice:   price,
		BuyAmount:      new(big.Int).Set(amountInt), // 使用Set来复制，避免指针问题
		RemainingCoin:  new(big.Int).Set(amountInt),
		Status:         StatusBought,
		BuyTime:        now,
		LastUpdateTime: now,
		Decimals:       decimals,
		RecentPrices: make([]struct {
			Price     float64
			Timestamp time.Time
		}, 0),
		// 初始化聚合周期开始时间，可以用当前时间截断，或保持零值由聚合器首次填充
		// 为简单起见，首次聚合时会自动填充
	}
	t.priceTracks[tokenAddress].RecentPrices = append(t.priceTracks[tokenAddress].RecentPrices, struct {
		Price     float64
		Timestamp time.Time
	}{price, now})

	// 启动价格聚合和原始价格更新协程
	go t.priceAggregatorLoop(tokenAddress)
	// 启动策略检查协程
	go t.trackPriceStrategy(tokenAddress)

	return nil
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

	// 更新 RecentPrices：维护一个固定长度（如100）的最新价格队列
	track.RecentPrices = append(track.RecentPrices, struct {
		Price     float64
		Timestamp time.Time // Timestamp 仍然保留，可能对其他分析有用或调试
	}{newRawPrice, time.Now()})

	// 如果队列超长，则从头部移除旧元素，保持固定长度
	if len(track.RecentPrices) > maxRecentPrices {
		track.RecentPrices = track.RecentPrices[len(track.RecentPrices)-maxRecentPrices:]
	}

	// 更新基于原始价格的历史最高价
	if newRawPrice > track.HighestPrice {
		track.HighestPrice = newRawPrice
		log.Printf("代币 %s 原始价格创新高: %.6f", track.Symbol, newRawPrice)
	}
}

// priceAggregatorLoop 模拟从WebSocket接收原始价格，并进行聚合
func (t *TradeExecutor) priceAggregatorLoop(tokenAddress string) {
	// 模拟价格流的ticker，例如每秒一个新价格
	priceStreamTicker := time.NewTicker(1 * time.Second)
	defer priceStreamTicker.Stop()

	for {
		select {
		case <-t.ctx.Done():
			log.Printf("价格聚合器 %s 停止", tokenAddress)
			return
		case now := <-priceStreamTicker.C:
			t.mutex.RLock() // RLock for reading priceTracks map
			track, exists := t.priceTracks[tokenAddress]
			t.mutex.RUnlock()

			if !exists {
				log.Printf("价格聚合器 %s 发现代币不再跟踪，停止", tokenAddress)
				return
			}

			// 1. 模拟产生新的原始价格
			var newRawPrice float64
			track.mutex.Lock() // Lock for reading track.CurrentPrice safely
			// 模拟价格小幅波动
			priceDirection := math.Sin(float64(now.UnixNano()/1e9) / 10.0) // Slow sine wave for direction
			changeFactor := 1 + (priceDirection * 0.005)                   // +/- 0.5% change
			newRawPrice = track.CurrentPrice * changeFactor
			if newRawPrice <= 0 { // Prevent price from becoming zero or negative in simulation
				newRawPrice = track.EntryPrice * 0.1 // Reset to some small value if it crashes
			}
			track.mutex.Unlock()

			// 2. 更新原始价格相关数据 (CurrentPrice, HighestPrice, StopLossLevel, RecentPrices)

			// 3. 执行价格聚合 - K线周期对齐逻辑
			track.mutex.Lock()
			nowAggTime := time.Now() // 聚合逻辑使用的时间戳

			// 15秒聚合周期
			currentCycle15s := nowAggTime.Truncate(15 * time.Second)
			if track.LastAggregatedCycleStart15s.IsZero() || currentCycle15s.After(track.LastAggregatedCycleStart15s) {
				track.LatestPrice15s = newRawPrice // 使用当前原始价格作为本周期的快照
				track.LastAggregatedCycleStart15s = currentCycle15s
				// log.Printf("聚合 %s 15s价格 (周期 %s): %.6f", tokenAddress, currentCycle15s.Format("15:04:05"), newRawPrice)
			}

			// 30秒聚合周期
			currentCycle30s := nowAggTime.Truncate(30 * time.Second)
			if track.LastAggregatedCycleStart30s.IsZero() || currentCycle30s.After(track.LastAggregatedCycleStart30s) {
				track.LatestPrice30s = newRawPrice
				track.LastAggregatedCycleStart30s = currentCycle30s
			}

			// 1分钟聚合周期
			currentCycle1m := nowAggTime.Truncate(1 * time.Minute)
			if track.LastAggregatedCycleStart1m.IsZero() || currentCycle1m.After(track.LastAggregatedCycleStart1m) {
				track.LatestPrice1m = newRawPrice
				track.LastAggregatedCycleStart1m = currentCycle1m
				log.Printf("聚合 %s 1m价格 (周期 %s): %.6f", tokenAddress, currentCycle1m.Format("15:04"), newRawPrice)
			}

			// 5分钟聚合周期
			currentCycle5m := nowAggTime.Truncate(5 * time.Minute)
			if track.LastAggregatedCycleStart5m.IsZero() || currentCycle5m.After(track.LastAggregatedCycleStart5m) {
				track.LatestPrice5m = newRawPrice
				track.LastAggregatedCycleStart5m = currentCycle5m
			}
			track.mutex.Unlock()
		}
	}
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
	triggeredStop, dropReason, actualDropPct := t.checkFixedQueueFallStop(track, 0.05)
	if triggeredStop {
		sellAmount := track.RemainingCoin
		log.Printf("固定队列止损触发 (%s): %s 价格跌幅 %.2f%%, 全部卖出 (约 %.6f 币)",
			dropReason, track.Symbol, actualDropPct*100, sellAmount)
		t.executeTokenSellInternal(track, tokenAddress, sellAmount)
		return
	}

	// 1. 新的兜底止损策略：如果当前最新原始价格低于买入价的5%
	if track.CurrentPrice < track.EntryPrice*0.95 {
		sellAmount := track.RemainingCoin
		lossPct := (track.EntryPrice - track.CurrentPrice) / track.EntryPrice * 100
		log.Printf("兜底止损触发: %s 当前价格 %.6f 低于买入价 %.6f 的 %.2f%%, 全部卖出 (约 %.6f 币)",
			track.Symbol, track.CurrentPrice, track.EntryPrice, lossPct, sellAmount)
		t.executeTokenSellInternal(track, tokenAddress, sellAmount)
		return
	}

	// 2. 检查止盈条件 - 使用 track.CurrentPrice 以便测试中能精确控制策略检查所用的价格
	if t.checkTakeProfit(track, tokenAddress, track.CurrentPrice) {
		return
	}
}

// 检查固定长度队列的头尾价格跌幅
func (t *TradeExecutor) checkFixedQueueFallStop(track *PriceTrackInfo, fallPercent float64) (triggered bool, reason string, actualFallPct float64) {
	queueLen := len(track.RecentPrices)
	if queueLen < maxRecentPrices { // 队列未满，不执行此止损，或根据需求调整逻辑
		return false, "队列未满", 0
	}

	headPrice := track.RecentPrices[0].Price          // 队列中最早的价格
	tailPrice := track.RecentPrices[queueLen-1].Price // 队列中最新的价格 (等同于 track.CurrentPrice)

	if headPrice > 0 && tailPrice < headPrice { // 必须有实际的下跌，且最早价格为正
		dropPct := (headPrice - tailPrice) / headPrice
		if dropPct >= fallPercent { // 跌幅达到指定百分比
			return true, fmt.Sprintf("最近%d个价格点内跌幅%.2f%%", maxRecentPrices, fallPercent*100), dropPct
		}
	}
	return false, "", 0
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

	// 已经卖出的币量占总买入量的百分比
	var soldPctOfBuyAmount float64
	if track.BuyAmount.Cmp(big.NewInt(0)) > 0 { // track.BuyAmount > 0
		buyAmountF := new(big.Float).SetInt(track.BuyAmount)
		remainingCoinF := new(big.Float).SetInt(track.RemainingCoin)
		soldAmountF := new(big.Float).Sub(buyAmountF, remainingCoinF)
		soldPctOfBuyAmountF := new(big.Float).Quo(soldAmountF, buyAmountF)
		soldPctOfBuyAmount, _ = soldPctOfBuyAmountF.Float64()
	}

	// 本次需要卖出的百分比 = 目标总卖出百分比 - 已卖出百分比
	sellPctForThisLevel := targetOverallSellPct - soldPctOfBuyAmount
	if sellPctForThisLevel <= 1e-6 { // 允许小的浮点误差
		return false
	}

	// 计算本次实际卖出数量 (基于初始购买量)
	sellAmountF := new(big.Float).SetInt(track.BuyAmount)
	sellPctF := new(big.Float).SetFloat64(sellPctForThisLevel)
	actualSellAmountF := new(big.Float).Mul(sellAmountF, sellPctF)

	sellAmount := new(big.Int)
	actualSellAmountF.Int(sellAmount) // 截断取整

	// 如果是100%目标卖出级别，则确保卖出所有剩余代币以处理精度问题
	if targetOverallSellPct == 1.0 {
		sellAmount.Set(track.RemainingCoin)
	} else {
		// 对于部分卖出，确保不超过剩余量
		if sellAmount.Cmp(track.RemainingCoin) > 0 {
			sellAmount.Set(track.RemainingCoin)
		}
	}

	// 如果最终计算的卖出数量为0（可能由于sellPctForThisLevel太小或已满足条件），
	// 则不执行卖出，并取消该级别的触发标记
	if sellAmount.Cmp(big.NewInt(0)) <= 0 {
		t.mutex.Lock()                      // Lock for t.triggeredLevels
		delete(t.triggeredLevels, levelKey) // 撤销标记
		t.mutex.Unlock()
		return false
	}

	log.Printf("止盈触发: %s 使用策略价格 %.6f (盈利 %.2f%%), 达到级别 %.2f%%. 目标总卖出 %.2f%%, 本次卖出初始购买量的 %.2f%% (%s 最小单位)",
		track.Symbol, priceForStrategy, currentPriceIncreasePct*100, targetOverallSellPct*100, targetOverallSellPct*100, sellPctForThisLevel*100, sellAmount.String())

	t.executeTokenSellInternal(track, tokenAddress, sellAmount) // Pass track
	return true
}

// 此函数在调用时，track 应已被锁定
func (t *TradeExecutor) executeTokenSellInternal(track *PriceTrackInfo, tokenAddress string, amount *big.Int) { // amount 类型改为 *big.Int
	if amount.Cmp(big.NewInt(0)) <= 0 { // 避免卖出0或负数数量
		log.Printf("尝试卖出 %s 的数量 %s 过小或为0，取消卖出", track.Symbol, amount.String())
		return
	}

	log.Printf("模拟执行卖出: 代币 %s, 数量 (最小单位): %s, 当前策略价格: %.6f, 当前原始价格: %.6f", track.Symbol, amount.String(), track.LatestPrice1m, track.CurrentPrice)

	track.RemainingCoin.Sub(track.RemainingCoin, amount)
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
	track.mutex.Lock()
	track.LatestPrice1m = newPrice
	track.mutex.Unlock()

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
