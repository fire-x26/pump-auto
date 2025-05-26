package execctor

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"pump_auto/internal/chainTx"
	"pump_auto/internal/common"
	"pump_auto/internal/ws"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// 代币交易状态
type TokenTradeStatus int

const (
	StatusNone    TokenTradeStatus = iota
	StatusBought                   // 已买入
	StatusSelling                  // 卖出中
	StatusSold                     // 已卖出
)

const (
	maxRecentPrices = 100 // 定义固定队列的长度
	PRECISION       = 16  // 价格计算精度
)

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
		common.Log.WithFields(logrus.Fields{
			"token": tokenAddress,
		}).Info("代币已经在跟踪列表中")
		return
	}

	var initialPrice = solToSpend / OutAmount
	initialPrice = math.Round(initialPrice*math.Pow10(PRECISION)) / math.Pow10(PRECISION)

	t.priceTracks[tokenAddress] = &PriceTrackInfo{
		Mint:           tokenAddress,
		EntryPrice:     initialPrice,
		HighestPrice:   initialPrice,
		CurrentPrice:   initialPrice,
		BuyAmount:      OutAmount,
		RemainingCoin:  OutAmount,
		SoldPercent:    0,
		Status:         StatusBought,
		BuyTime:        time.Now(),
		LastUpdateTime: time.Now(),
		mutex:          sync.Mutex{},
	}

	// 使用WithFields记录结构体的各个字段
	common.Log.WithFields(logrus.Fields{
		"token":         tokenAddress,
		"entryPrice":    initialPrice,
		"highestPrice":  initialPrice,
		"currentPrice":  initialPrice,
		"buyAmount":     OutAmount,
		"remainingCoin": OutAmount,
		"soldPercent":   0,
		"status":        StatusBought,
		"buyTime":       time.Now(),
	}).Debug("代币追踪初始化完成")

	common.Log.WithFields(logrus.Fields{
		"token":        tokenAddress,
		"solAmount":    solToSpend,
		"initialPrice": initialPrice,
	}).Info("等待买入交易消息")
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
		common.Log.Info(fmt.Sprintf("代币--%s,价格创新高 %s ", tokenAddress, newRawPrice))
	}
}

// 检查并执行策略(止盈/止损)
func (t *TradeExecutor) checkAndExecuteStrategies(track *PriceTrackInfo, tokenAddress string) {
	track.mutex.Lock()
	defer track.mutex.Unlock()

	// 1. 新的兜底止损策略：如果当前最新原始价格低于买入价的5%
	if track.CurrentPrice < track.EntryPrice*0.95 {
		common.Log.Debug(fmt.Sprintf("进入兜底止损，token--%s,当前价格--%s,买入价格--%s", tokenAddress, track.CurrentPrice, track.EntryPrice))

		t.executeTokenSellInternal(track, track.SoldPercent, tokenAddress, track.RemainingCoin, fmt.Sprintf("%.0f%%", 1.00*100), false, 20, 0.0005, common.PUMP)

		return
	}

	// 2. 检查止盈条件
	common.Log.Debug("开始检查止盈条件")
	if t.checkTakeProfit(track, tokenAddress, track.CurrentPrice) {
		common.Log.Info("止盈条件已触发")
		return
	}
	common.Log.Debug("止盈条件未触发")
}

// 检查并执行止盈策略 - 使用switch-case提高效率
// track 应已被外部锁定
func (t *TradeExecutor) checkTakeProfit(track *PriceTrackInfo, tokenAddress string, priceForStrategy float64) bool {

	// 计算价格涨幅，使用16位精度
	currentPriceIncreasePct := (priceForStrategy - track.EntryPrice) / track.EntryPrice
	currentPriceIncreasePct = math.Round(currentPriceIncreasePct*math.Pow10(PRECISION)) / math.Pow10(PRECISION)

	common.Log.WithFields(logrus.Fields{
		"token":            track.Mint,
		"currentPrice":     priceForStrategy,
		"entryPrice":       track.EntryPrice,
		"priceIncrease":    currentPriceIncreasePct * 100,
		"soldedPercentage": track.SoldPercent * 100,
	}).Info("价格检查详情")

	levelKey := ""
	targetOverallSellPct := 0.0 // 目标总共卖出的原始购买量的百分比
	const epsilon = 1e-8        // 定义一个小的容差值

	switch {
	case currentPriceIncreasePct >= (1.00 - epsilon):
		levelKey = fmt.Sprintf("%s_100", tokenAddress)
		targetOverallSellPct = 1.00
		common.Log.Info("触发 100%% 止盈点")
	case currentPriceIncreasePct >= (0.90 - epsilon):
		levelKey = fmt.Sprintf("%s_90", tokenAddress)
		targetOverallSellPct = 0.90
		common.Log.Info("触发 90%% 止盈点")
	case currentPriceIncreasePct >= (0.80 - epsilon):
		levelKey = fmt.Sprintf("%s_80", tokenAddress)
		targetOverallSellPct = 0.80
		common.Log.Info("触发 80%% 止盈点")
	case currentPriceIncreasePct >= (0.70 - epsilon):
		levelKey = fmt.Sprintf("%s_70", tokenAddress)
		targetOverallSellPct = 0.70
		common.Log.Info("触发 70%% 止盈点")
	case currentPriceIncreasePct >= (0.60 - epsilon):
		levelKey = fmt.Sprintf("%s_60", tokenAddress)
		targetOverallSellPct = 0.60
		common.Log.Info("触发 60%% 止盈点")
	case currentPriceIncreasePct >= (0.50 - epsilon):
		levelKey = fmt.Sprintf("%s_50", tokenAddress)
		targetOverallSellPct = 0.50
		common.Log.Info("触发 50%% 止盈点")
	case currentPriceIncreasePct >= (0.40 - epsilon):
		levelKey = fmt.Sprintf("%s_40", tokenAddress)
		targetOverallSellPct = 0.40
		common.Log.Info("触发 40%% 止盈点")
	case currentPriceIncreasePct >= (0.30 - epsilon):
		levelKey = fmt.Sprintf("%s_30", tokenAddress)
		targetOverallSellPct = 0.30
		common.Log.Info("触发 30%% 止盈点")
	case currentPriceIncreasePct >= (0.20 - epsilon):
		levelKey = fmt.Sprintf("%s_20", tokenAddress)
		targetOverallSellPct = 0.20
		common.Log.Info("触发 20%% 止盈点")
	case currentPriceIncreasePct >= (0.10 - epsilon):
		levelKey = fmt.Sprintf("%s_10", tokenAddress)
		targetOverallSellPct = 0.10
		common.Log.Info("触发 10%% 止盈点")
	default:
		common.Log.Info("未达到任何止盈点")
		return false
	}

	if targetOverallSellPct <= track.SoldPercent {
		common.Log.WithFields(logrus.Fields{
			"currentLevel":     levelKey,
			"soldPercentage":   track.SoldPercent * 100,
			"targetPercentage": targetOverallSellPct * 100,
		}).Warn("当前level 但已卖出比例 大于目标比例")
		return true
	}

	sellAmount := track.BuyAmount * (targetOverallSellPct - track.SoldPercent)
	common.Log.WithFields(logrus.Fields{
		"token":                tokenAddress,
		"targetPercentage":     targetOverallSellPct * 100,
		"currentPercentage":    track.SoldPercent * 100,
		"sellAmount":           sellAmount,
		"buyAmount":            track.BuyAmount,
		"targetOverallSellPct": targetOverallSellPct,
		"SoldPercent":          track.SoldPercent,
	}).Info("准备执行卖出")

	t.executeTokenSellInternal(track, targetOverallSellPct, tokenAddress, sellAmount,
		fmt.Sprintf("%.0f%%", targetOverallSellPct*100), false, 20, 0.0005, common.PUMP)
	return true
}

// 此函数在调用时，track 应已被锁定
func (t *TradeExecutor) executeTokenSellInternal(track *PriceTrackInfo, SoldPercent float64, tokenAddress string, sellAmount float64, sellPercent string, denominatedInSol bool, slippage int, priorityFee float64, poolType common.PoolType) {

	if sellAmount <= 0 { // 避免卖出0或负数数量
		common.Log.Warn("尝试卖出的数量过小或为0，取消卖出")
		return
	}

	_, err := chainTx.SellToken(tokenAddress, sellAmount, sellPercent, denominatedInSol, slippage, priorityFee, poolType)
	if err != nil {
		common.Log.WithError(err).Error("卖出代币失败")
	}

	if sellPercent == "100%" {
		// 发送取消订阅消息
		if err := ws.UnsubscribeToTokenTrades([]string{tokenAddress}); err != nil {
			common.Log.WithError(err).Error("取消订阅代币失败")
		} else {
			common.Log.Info("成功取消订阅代币的交易事件")
			// 调用回调函数通知Bot移除代币
			if t.onTokenSold != nil {
				t.onTokenSold(tokenAddress)
			}
		}
		return
	}

	common.Log.Info("执行卖出")

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
	common.Log.Info("交易执行器已停止")
}

// ProcessTradeMessage 处理从WebSocket收到的交易消息
func (t *TradeExecutor) ProcessTradeMessage(message []byte) {
	var tradeRecord TradeRecord
	if err := json.Unmarshal(message, &tradeRecord); err != nil {
		common.Log.WithFields(logrus.Fields{
			"error": err,
			"raw":   string(message),
		}).Error("解析WebSocket交易消息失败")
		return
	}
	logger := common.Log.WithFields(logrus.Fields{})
	logger.Debug(fmt.Sprintf("接收到交易消息,detail,%s", tradeRecord))

	t.mutex.RLock()
	track, exists := t.priceTracks[tradeRecord.Mint]
	t.mutex.RUnlock()

	if !exists {
		return // Token not expected at all
	}

	// 使用16位精度处理价格计算
	price := tradeRecord.SolAmount / tradeRecord.TokenAmount
	price = math.Round(price*math.Pow10(PRECISION)) / math.Pow10(PRECISION)

	// 检查价格是否有效
	if math.IsInf(price, 0) || math.IsNaN(price) || price <= 0 {
		logger.Warn(fmt.Sprintf("价格--%s，处理无效，跳过处理", price))
		return
	}

	logger.Debug(fmt.Sprintf("计算得到新价格--%s", price))

	// 只要代币在我们关注列表（不论状态是None, Bought, Selling），都更新其当前价格信息
	t.UpdatePrice(tradeRecord.Mint, price)

	// 检查代币状态并执行策略
	t.mutex.RLock()
	track, exists = t.priceTracks[tradeRecord.Mint]
	if exists && track.Status != StatusSold {
		t.mutex.RUnlock()
		logger.Debug("开始检查策略")
		t.checkAndExecuteStrategies(track, tradeRecord.Mint)
		logger.Debug("策略检查完毕 ---- ", tradeRecord.Mint)
	} else {
		t.mutex.RUnlock()
	}
}
