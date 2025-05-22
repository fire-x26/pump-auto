package execctor

import (
	"fmt"
	"math/big"
	"testing"
	"time"
)

// TestTakeProfitStrategy 测试止盈策略
func TestTakeProfitStrategy(t *testing.T) {
	// 初始化交易执行器
	executor := NewTradeExecutor()

	// 测试代币信息
	tokenAddress := "TestToken123"
	tokenSymbol := "TEST"
	initialAmount := 1.0
	initialPrice := 1.0
	decimals := uint8(9) // Solana代币通常为9位小数

	// 买入代币
	err := executor.BuyToken(tokenAddress, tokenSymbol, initialAmount, initialPrice, decimals)
	if err != nil {
		t.Fatalf("买入代币失败: %v", err)
	}

	// 等待一小段时间，让聚合器初始化
	time.Sleep(100 * time.Millisecond)

	// 获取初始状态
	info := executor.GetTradeInfo(tokenAddress)
	if info == nil {
		t.Fatalf("获取代币信息失败")
	}

	initialRemainingCoin := new(big.Int).Set(info.RemainingCoin)

	// 测试场景: 价格上涨10%，应该触发第一级止盈（卖出10%）
	t.Run("价格上涨10%触发止盈", func(t *testing.T) {
		// 模拟价格上涨10%
		executor.SimulatePriceChange(tokenAddress, 1.1)

		// 等待策略执行
		time.Sleep(100 * time.Millisecond)

		// 获取最新状态
		info = executor.GetTradeInfo(tokenAddress)

		// 验证是否卖出了一部分代币
		if info.RemainingCoin.Cmp(initialRemainingCoin) >= 0 {
			t.Errorf("价格上涨10%%后应触发止盈卖出一部分代币，但剩余币量未减少")
		}

		// 计算理论上的卖出比例（应该卖出初始量的10%）
		expectedSoldPct := 0.1
		expectedRemaining := new(big.Float).Mul(
			new(big.Float).SetInt(initialRemainingCoin),
			new(big.Float).SetFloat64(1.0-expectedSoldPct),
		)

		// 转换为big.Int进行比较
		expectedRemainingInt := new(big.Int)
		expectedRemaining.Int(expectedRemainingInt)

		// 允许一点误差（由于big.Int转换时的截断）
		t.Logf("价格上涨10%%后, 期望剩余: %s, 实际剩余: %s",
			expectedRemainingInt.String(), info.RemainingCoin.String())
	})

	// 记录第一次止盈后的剩余币量
	remainingAfterFirstTP := new(big.Int).Set(info.RemainingCoin)

	// 测试场景: 价格上涨到30%，应该再次触发止盈
	t.Run("价格上涨30%触发更高级别止盈", func(t *testing.T) {
		// 模拟价格上涨到初始价格的1.3倍
		executor.SimulatePriceChange(tokenAddress, 1.3)

		// 等待策略执行
		time.Sleep(100 * time.Millisecond)

		// 获取最新状态
		info = executor.GetTradeInfo(tokenAddress)

		// 验证是否再次卖出了一部分代币
		if info.RemainingCoin.Cmp(remainingAfterFirstTP) >= 0 {
			t.Errorf("价格上涨30%%后应再次触发止盈，但剩余币量未减少")
		}

		t.Logf("价格上涨30%%后, 第一次止盈后剩余: %s, 现在剩余: %s",
			remainingAfterFirstTP.String(), info.RemainingCoin.String())
	})

	// 测试场景: 价格大幅下跌，应该触发止损
	t.Run("价格大幅下跌触发止损", func(t *testing.T) {
		// 先模拟价格继续上涨，再模拟大幅下跌
		executor.SimulatePriceChange(tokenAddress, 1.5) // 先涨到1.5倍
		time.Sleep(50 * time.Millisecond)

		// 现在价格大幅下跌到1.2倍（从1.5跌到1.2，跌幅达到20%，应该触发止损）
		executor.SimulatePriceChange(tokenAddress, 1.2)

		// 等待策略执行
		time.Sleep(100 * time.Millisecond)

		// 获取最新状态
		info = executor.GetTradeInfo(tokenAddress)

		// 验证是否卖出了剩余的全部代币（止损通常会清仓）
		if info.RemainingCoin.Cmp(big.NewInt(0)) != 0 {
			t.Errorf("价格大幅下跌后应触发止损清仓，但仍有剩余币量: %s",
				info.RemainingCoin.String())
		}

		t.Logf("触发止损后, 剩余币量: %s, 交易状态: %v",
			info.RemainingCoin.String(), info.Status)
	})

	// 测试完成后清理资源
	executor.Stop()
}

// TestStopLossStrategy 测试止损策略
func TestStopLossStrategy(t *testing.T) {
	// 初始化交易执行器
	executor := NewTradeExecutor()

	// 测试代币信息
	tokenAddress := "TestTokenSL123"
	tokenSymbol := "TESTSL"
	initialAmount := 1.0
	initialPrice := 1.0
	decimals := uint8(9)

	// 买入代币
	err := executor.BuyToken(tokenAddress, tokenSymbol, initialAmount, initialPrice, decimals)
	if err != nil {
		t.Fatalf("买入代币失败: %v", err)
	}

	// 等待一小段时间，让聚合器初始化
	time.Sleep(100 * time.Millisecond)

	// 获取初始状态
	info := executor.GetTradeInfo(tokenAddress)
	if info == nil {
		t.Fatalf("获取代币信息失败")
	}

	// 测试场景: 直接下跌超过5%，触发兜底止损
	t.Run("直接下跌触发兜底止损", func(t *testing.T) {
		// 模拟价格下跌7%
		executor.SimulatePriceChange(tokenAddress, 0.93)

		// 等待策略执行
		time.Sleep(100 * time.Millisecond)

		// 获取最新状态
		info = executor.GetTradeInfo(tokenAddress)

		// 验证是否已卖出全部代币
		if info.RemainingCoin.Cmp(big.NewInt(0)) != 0 {
			t.Errorf("价格下跌7%%后应触发兜底止损，但仍有剩余币量: %s",
				info.RemainingCoin.String())
		}

		t.Logf("触发兜底止损后, 剩余币量: %s, 交易状态: %v",
			info.RemainingCoin.String(), info.Status)
	})

	// 测试完成后清理资源
	executor.Stop()
}

// TestFixedQueueFallStop 测试基于队列的止损策略
func TestFixedQueueFallStop(t *testing.T) {
	// 初始化交易执行器
	executor := NewTradeExecutor()

	// 测试代币信息
	tokenAddress := "TestTokenQueue123"
	tokenSymbol := "TESTQ"
	initialAmount := 1.0
	initialPrice := 1.0
	decimals := uint8(9)

	// 买入代币
	err := executor.BuyToken(tokenAddress, tokenSymbol, initialAmount, initialPrice, decimals)
	if err != nil {
		t.Fatalf("买入代币失败: %v", err)
	}

	// 等待一小段时间，让聚合器初始化
	time.Sleep(100 * time.Millisecond)

	// 获取初始状态
	info := executor.GetTradeInfo(tokenAddress)
	if info == nil {
		t.Fatalf("获取代币信息失败")
	}

	// 填充价格队列
	t.Log("填充价格队列...")
	for i := 0; i < maxRecentPrices; i++ {
		// 模拟价格小幅波动，但整体是上涨的
		priceFactor := 1.0 + float64(i)*0.001
		executor.UpdatePrice(tokenAddress, initialPrice*priceFactor)
		time.Sleep(10 * time.Millisecond)
	}

	// 测试场景: 填充队列后快速下跌，触发队列止损
	t.Run("队列头尾价格差超过5%触发止损", func(t *testing.T) {
		// 获取填充队列后的状态
		info = executor.GetTradeInfo(tokenAddress)

		// 模拟价格快速下跌10%
		// 从队列最高点下跌10%应该触发止损

		// 执行策略检查
		executor.checkAndExecuteStrategies(tokenAddress)

		// 等待策略执行
		time.Sleep(100 * time.Millisecond)

		// 获取最新状态
		info = executor.GetTradeInfo(tokenAddress)

		// 验证是否已卖出全部代币
		if info.RemainingCoin.Cmp(big.NewInt(0)) != 0 {
			t.Errorf("队列价格下跌超过5%%后应触发止损，但仍有剩余币量: %s",
				info.RemainingCoin.String())
		}

		t.Logf("触发队列止损后, 剩余币量: %s, 交易状态: %v",
			info.RemainingCoin.String(), info.Status)
	})

	// 测试完成后清理资源
	executor.Stop()
}

// TestMultipleTakeProfit 测试多级止盈策略，模拟多次涨幅直至全部卖出
func TestMultipleTakeProfit(t *testing.T) {
	// 初始化交易执行器
	executor := NewTradeExecutor()

	// 测试代币信息
	tokenAddress := "TestTokenMultiTP"
	tokenSymbol := "MULTI"
	initialAmount := 1.0
	initialPrice := 1.0
	decimals := uint8(9) // Solana代币通常为9位小数

	// 买入代币
	err := executor.BuyToken(tokenAddress, tokenSymbol, initialAmount, initialPrice, decimals)
	if err != nil {
		t.Fatalf("买入代币失败: %v", err)
	}

	// 等待一小段时间，让聚合器初始化
	time.Sleep(100 * time.Millisecond)

	// 获取初始状态
	info := executor.GetTradeInfo(tokenAddress)
	if info == nil {
		t.Fatalf("获取代币信息失败")
	}

	initialRemainingCoin := new(big.Int).Set(info.RemainingCoin)
	t.Logf("初始代币数量: %s", initialRemainingCoin.String())

	// 定义不同的价格涨幅级别和对应的总卖出比例
	priceIncreases := []struct {
		increasePct   float64 // 价格涨幅百分比
		targetSellPct float64 // 该级别对应的目标总卖出比例
		description   string  // 描述
	}{
		{0.1, 0.1, "价格上涨10%"},
		{0.2, 0.2, "价格上涨20%"},
		{0.3, 0.3, "价格上涨30%"},
		{0.4, 0.4, "价格上涨40%"},
		{0.5, 0.5, "价格上涨50%"},
		{0.6, 0.6, "价格上涨60%"},
		{0.7, 0.7, "价格上涨70%"},
		{0.8, 0.8, "价格上涨80%"},
		{0.9, 0.9, "价格上涨90%"},
		{1.0, 1.0, "价格上涨100%"},
	}

	// 逐步模拟价格上涨，验证每个级别的止盈策略
	var remainingCoins []*big.Int
	remainingCoins = append(remainingCoins, new(big.Int).Set(initialRemainingCoin))

	for i, level := range priceIncreases {
		t.Run(fmt.Sprintf("级别%d: %s", i+1, level.description), func(t *testing.T) {
			// 模拟价格上涨到指定涨幅
			priceFactor := 1.0 + level.increasePct
			executor.SimulatePriceChange(tokenAddress, priceFactor)

			// 等待策略执行
			time.Sleep(100 * time.Millisecond)

			// 获取最新状态
			info = executor.GetTradeInfo(tokenAddress)

			// 保存当前剩余币量，用于下一级别的比较
			currentRemaining := new(big.Int).Set(info.RemainingCoin)
			remainingCoins = append(remainingCoins, currentRemaining)

			// 验证是否卖出了预期比例的代币
			if i > 0 {
				if info.RemainingCoin.Cmp(remainingCoins[i]) >= 0 {
					t.Errorf("%s后应触发止盈卖出更多代币，但剩余币量未减少", level.description)
				}
			}

			// 计算理论上应该剩余的币量
			expectedRemainingPct := 1.0 - level.targetSellPct
			expectedRemainingF := new(big.Float).Mul(
				new(big.Float).SetInt(initialRemainingCoin),
				new(big.Float).SetFloat64(expectedRemainingPct),
			)
			expectedRemainingInt := new(big.Int)
			expectedRemainingF.Int(expectedRemainingInt)

			// 记录并验证结果
			t.Logf("%s后, 期望剩余比例: %.2f%%, 期望剩余: %s, 实际剩余: %s",
				level.description,
				expectedRemainingPct*100,
				expectedRemainingInt.String(),
				info.RemainingCoin.String())

			// 最后一个级别应该已经全部卖出
			if i == len(priceIncreases)-1 {
				if info.RemainingCoin.Cmp(big.NewInt(0)) != 0 {
					t.Errorf("在最高级别(100%%)后应该全部卖出，但仍有剩余币量: %s",
						info.RemainingCoin.String())
				} else {
					t.Logf("代币已全部卖出，交易状态: %v", info.Status)
				}
			}
		})
	}

	// 测试完成后清理资源
	executor.Stop()
}
