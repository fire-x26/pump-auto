package chainTx

import (
	"pump_auto/internal/common"
	"testing"

	"github.com/gagliardetto/solana-go"
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
			mint:             "2kicCkhMhte7k2eqGgWkfPhCbV1EnSnYktxq8HtGpump", // SOL代币地址
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

func TestGetTokenBalance(t *testing.T) {
	tests := []struct {
		name    string
		mint    string
		want    float64
		wantErr bool
	}{
		{
			name:    "正常代币余额查询",
			mint:    "J6ZevyEKWf6NS9LsJNNmKVyKn3fv7sZmuGGBtQJWpump", // 使用测试用例中的代币地址
			want:    0,                                              // 由于余额会变化，我们只检查是否能成功获取
			wantErr: false,
		},
		{
			name:    "无效代币地址",
			mint:    "invalid_mint_address",
			want:    0,
			wantErr: true,
		},
		{
			name:    "不存在的代币",
			mint:    "11111111111111111111111111111111",
			want:    0,
			wantErr: false, // 不存在的代币应该返回0而不是错误
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetTokenBalance(tt.mint)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetTokenBalance() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got < 0 {
				t.Error("GetTokenBalance() 返回的余额不能为负数")
			}
			// 打印余额信息用于调试
			t.Logf("代币 %s 的余额为: %f", tt.mint, got)
		})
	}
}

func TestGetTokenDecimal(t *testing.T) {
	tests := []struct {
		name    string
		mint    string
		want    uint8
		wantErr bool
	}{
		{
			name:    "正常代币精度查询",
			mint:    "9BVFGcsfTkf27ZbHEuGoGanfacy7ntWZNW25xhcHpump", // 使用测试用例中的代币地址
			want:    9,                                              // 大多数代币的精度是9
			wantErr: false,
		},
		{
			name:    "无效代币地址",
			mint:    "invalid_mint_address",
			want:    0,
			wantErr: true,
		},
		{
			name:    "不存在的代币",
			mint:    "11111111111111111111111111111111",
			want:    0,
			wantErr: true, // 不存在的代币应该返回错误
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetTokenDecimal(tt.mint)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetTokenDecimal() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("GetTokenDecimal() = %v, want %v", got, tt.want)
			}
			// 打印精度信息用于调试
			t.Logf("代币 %s 的精度为: %d", tt.mint, got)
		})
	}
}

func TestParseTxSign(t *testing.T) {
	tests := []struct {
		name    string
		txHash  string
		wantErr bool
	}{
		{
			name:    "正常交易哈希",
			txHash:  "4APVY1su7T5RqEBZQpHTHxqUbm6YvmboB8FLXnVtS7zqzQrb25tDCWJBSzwhjdEVRXM6HpiLC4G6tSA8r4J3iztB", // 请替换为真实可用的交易哈希
			wantErr: false,
		},
		{
			name:    "无效交易哈希",
			txHash:  "invalid_hash",
			wantErr: true,
		},
		{
			name:    "空交易哈希",
			txHash:  "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var err error
			if tt.txHash == "" {
				_, err = ParseTxSign(solana.Signature{})
			} else {
				sig, sigErr := solana.SignatureFromBase58(tt.txHash)
				if sigErr != nil {
					err = sigErr
				} else {
					_, err = ParseTxSign(sig)
				}
			}
			if (err != nil) != tt.wantErr {
				t.Errorf("parseTxSign() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
