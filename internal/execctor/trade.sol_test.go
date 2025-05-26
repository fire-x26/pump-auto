package execctor

import (
	"pump_auto/internal/common"
	"sync"
	"testing"
	"time"
)

func TestExecuteTokenSellInternal(t *testing.T) {
	// 创建一个测试用的TradeExecutor
	executor := NewTradeExecutor(func(tokenAddress string) {
		t.Logf("代币售出回调被触发: %s", tokenAddress)
	})

	// 测试用例1: 正常卖出场景
	t.Run("正常卖出", func(t *testing.T) {
		track := &PriceTrackInfo{
			Mint:           "test_token_1",
			EntryPrice:     1.0,
			HighestPrice:   1.0,
			CurrentPrice:   1.0,
			BuyAmount:      1000,
			RemainingCoin:  1000,
			SoldPercent:    0,
			Status:         StatusBought,
			BuyTime:        time.Now(),
			LastUpdateTime: time.Now(),
			mutex:          sync.Mutex{},
		}

		executor.executeTokenSellInternal(track, 0.1, "test_token_1", 100, "10%", false, 20, 0.0005, common.PUMP)

		if track.SoldPercent != 0.1 {
			t.Errorf("期望SoldPercent为0.1，实际为%f", track.SoldPercent)
		}
	})

	// 测试用例2: 卖出数量为0
	t.Run("卖出数量为0", func(t *testing.T) {
		track := &PriceTrackInfo{
			Mint:           "test_token_2",
			EntryPrice:     1.0,
			HighestPrice:   1.0,
			CurrentPrice:   1.0,
			BuyAmount:      1000,
			RemainingCoin:  1000,
			SoldPercent:    0,
			Status:         StatusBought,
			BuyTime:        time.Now(),
			LastUpdateTime: time.Now(),
			mutex:          sync.Mutex{},
		}

		executor.executeTokenSellInternal(track, 0.1, "test_token_2", 0, "10%", false, 20, 0.0005, common.PUMP)

		if track.SoldPercent != 0 {
			t.Errorf("期望SoldPercent保持为0，实际为%f", track.SoldPercent)
		}
	})

	// 测试用例3: 100%卖出
	t.Run("100%卖出", func(t *testing.T) {
		track := &PriceTrackInfo{
			Mint:           "test_token_3",
			EntryPrice:     1.0,
			HighestPrice:   1.0,
			CurrentPrice:   1.0,
			BuyAmount:      1000,
			RemainingCoin:  1000,
			SoldPercent:    0,
			Status:         StatusBought,
			BuyTime:        time.Now(),
			LastUpdateTime: time.Now(),
			mutex:          sync.Mutex{},
		}

		executor.executeTokenSellInternal(track, 1.0, "test_token_3", 1000, "100%", false, 20, 0.0005, common.PUMP)

		if track.SoldPercent != 1.0 {
			t.Errorf("期望SoldPercent为1.0，实际为%f", track.SoldPercent)
		}
	})

	// 测试用例4: 卖出数量超过剩余数量
	t.Run("卖出数量超过剩余数量", func(t *testing.T) {
		track := &PriceTrackInfo{
			Mint:           "test_token_4",
			EntryPrice:     1.0,
			HighestPrice:   1.0,
			CurrentPrice:   1.0,
			BuyAmount:      1000,
			RemainingCoin:  500,
			SoldPercent:    0.5,
			Status:         StatusBought,
			BuyTime:        time.Now(),
			LastUpdateTime: time.Now(),
			mutex:          sync.Mutex{},
		}

		executor.executeTokenSellInternal(track, 0.8, "test_token_4", 800, "80%", false, 20, 0.0005, common.PUMP)

		if track.SoldPercent != 0.8 {
			t.Errorf("期望SoldPercent为0.8，实际为%f", track.SoldPercent)
		}
	})
}
