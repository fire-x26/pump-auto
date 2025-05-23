package bot

import (
	"pump_auto/internal/common"
	"pump_auto/internal/model"
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

func TestFetchMetadata(t *testing.T) {
	tests := []struct {
		name    string
		uri     string
		wantErr bool
		want    *model.TokenMetadata // Expected metadata, can be nil if wantErr is true
	}{
		{
			name:    "Valid IPFS URI - Golden BTC Phrase",
			uri:     "https://ipfs.io/ipfs/Qmb1R1XDc1HDadcjpNS1Wdxc3uJmdyj7QZjzdfef9wifbi",
			wantErr: false,
			want: &model.TokenMetadata{
				Name:        "Golden BTC Phrase",
				Symbol:      "GOLDP",
				Description: "The True Golden standard.\\r\\nThe Golden ticket. \\r\\nWorth more than Gold…",
				Image:       "https://ipfs.io/ipfs/QmQY5bB3dyPJAc4k8QgNhWBuurTd8Hk8eTzHeT2ihDq7PE",
				ShowName:    true,
				CreatedOn:   "https://pump.fun",
			},
		},
		{
			name:    "Invalid URI - Non-existent",
			uri:     "https://ipfs.io/ipfs/THIS_HASH_DOES_NOT_EXIST_12345",
			wantErr: true,
			want:    nil,
		},
		{
			name:    "Invalid URI - Malformed",
			uri:     "not_a_valid_uri",
			wantErr: true,
			want:    nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := fetchMetadata(tt.uri)
			if (err != nil) != tt.wantErr {
				t.Errorf("fetchMetadata() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if got == nil {
					t.Errorf("fetchMetadata() got nil metadata, want non-nil")
					return
				}
				// Compare relevant fields
				if got.Name != tt.want.Name {
					t.Errorf("fetchMetadata() got Name = %v, want %v", got.Name, tt.want.Name)
				}
				if got.Symbol != tt.want.Symbol {
					t.Errorf("fetchMetadata() got Symbol = %v, want %v", got.Symbol, tt.want.Symbol)
				}
				// Be careful with multi-line descriptions or special characters if not matching exactly
				// For simplicity, we'll do a direct compare here. Consider normalizing if it becomes flaky.
				if got.Description != tt.want.Description {
					t.Errorf("fetchMetadata() got Description = %q, want %q", got.Description, tt.want.Description)
				}
				if got.Image != tt.want.Image {
					t.Errorf("fetchMetadata() got Image = %v, want %v", got.Image, tt.want.Image)
				}
				if got.ShowName != tt.want.ShowName {
					t.Errorf("fetchMetadata() got ShowName = %v, want %v", got.ShowName, tt.want.ShowName)
				}
				if got.CreatedOn != tt.want.CreatedOn {
					t.Errorf("fetchMetadata() got CreatedOn = %v, want %v", got.CreatedOn, tt.want.CreatedOn)
				}
			}
		})
	}
}
