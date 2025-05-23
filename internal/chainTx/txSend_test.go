package chainTx

import (
	"pump_auto/internal/common"
	"testing"
)

func TestBuyToken(t *testing.T) {
	tests := []struct {
		name             string
		mint             string
		amount           float64
		denominatedInSol bool
		slippage         int
		priorityFee      float64
		pool             common.PoolType
		wantErr          bool
	}{
		{
			name:             "正常买入测试",
			mint:             "32bKPLHRThqX7r67AvEJD1wccTmeKaWcMwofTDkBpump", // SOL代币地址
			amount:           0.010000,
			denominatedInSol: true,
			slippage:         10,
			priorityFee:      0.0005,
			pool:             "pump",
			wantErr:          false,
		},
		{
			name:             "无效代币地址测试",
			mint:             "invalid_mint_address",
			amount:           0.1,
			denominatedInSol: true,
			slippage:         1,
			priorityFee:      0.000005,
			pool:             common.RAYDIUM,
			wantErr:          true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			txHash, err := BuyToken(tt.mint, tt.amount, tt.denominatedInSol, tt.slippage, tt.priorityFee, tt.pool)
			if (err != nil) != tt.wantErr {
				t.Errorf("BuyToken() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && txHash == "" {
				t.Error("BuyToken() 返回的交易哈希为空")
			}
		})
	}
}

func TestSellToken(t *testing.T) {
	tests := []struct {
		name             string
		mint             string
		amount           float64
		sellPercent      string
		denominatedInSol bool
		slippage         int
		priorityFee      float64
		pool             common.PoolType
		wantErr          bool
	}{
		{
			name:             "正常卖出测试",
			mint:             "45Xxq2nrxLA2cy4AJKkKyiYqKyKVqcCtdy7j1kNqpump", // SOL代币地址
			amount:           1,
			sellPercent:      "100%",
			denominatedInSol: false,
			slippage:         10,
			priorityFee:      0.000005,
			pool:             common.PUMP,
			wantErr:          false,
		},
		{
			name:             "无效代币地址测试",
			mint:             "invalid_mint_address",
			amount:           0.1,
			sellPercent:      "100",
			denominatedInSol: true,
			slippage:         1,
			priorityFee:      0.000005,
			pool:             common.RAYDIUM,
			wantErr:          true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			txHash, err := SellToken(tt.mint, tt.amount, tt.sellPercent, tt.denominatedInSol, tt.slippage, tt.priorityFee, tt.pool)
			if (err != nil) != tt.wantErr {
				t.Errorf("SellToken() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && txHash == "" {
				t.Error("SellToken() 返回的交易哈希为空")
			}
		})
	}
}
