package bot

import (
	"pump_auto/internal/common"
	"testing"
)

func TestBot_buyToken(t *testing.T) {
	b := NewBot()

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
			mint:             "7kXwmx81UteinNHkCBRfVdZfiwMG8oyak824zUPDpump", // SOL主网mint
			amount:           0.00100,
			denominatedInSol: true,
			slippage:         10,
			priorityFee:      0.0005,
			pool:             common.PUMP,
			wantErr:          false,
		},
		{
			name:             "无效mint测试",
			mint:             "invalid_mint_address",
			amount:           0.01,
			denominatedInSol: true,
			slippage:         10,
			priorityFee:      0.0005,
			pool:             common.PUMP,
			wantErr:          true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sign, err := b.buyToken(tt.mint, tt.amount, tt.denominatedInSol, tt.slippage, tt.priorityFee, tt.pool)
			if (err != nil) != tt.wantErr {
				t.Errorf("buyToken() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && sign == "" {
				t.Error("buyToken() 返回的签名为空")
			}
		})
	}
}