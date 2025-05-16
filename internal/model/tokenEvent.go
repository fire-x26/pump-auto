package model

import (
	"encoding/json"
	"fmt"
)

type TokenEvent struct {
	Signature             string  `json:"signature"`
	Mint                  string  `json:"mint"`
	TraderPublicKey       string  `json:"traderPublicKey"`
	TxType                string  `json:"txType"`
	InitialBuy            float64 `json:"initialBuy"`
	SolAmount             float64 `json:"solAmount"`
	BondingCurveKey       string  `json:"bondingCurveKey"`
	VTokensInBondingCurve float64 `json:"vTokensInBondingCurve"`
	VSolInBondingCurve    float64 `json:"vSolInBondingCurve"`
	MarketCapSol          float64 `json:"marketCapSol"`
	Name                  string  `json:"name"`
	Symbol                string  `json:"symbol"`
	Uri                   string  `json:"uri"`
	Pool                  string  `json:"pool"`
}

// FormatTokenEvent 格式化显示代币事件信息
func FormatTokenEvent(data []byte) string {
	var event TokenEvent
	if err := json.Unmarshal(data, &event); err != nil {
		return fmt.Sprintf("解析消息失败: %v\n原始消息: %s", err, string(data))
	}

	return fmt.Sprintf(`
		==== 新代币事件信息 ====
		signature: %s
		mint: %s
		traderPublicKey: %s
		txType: %s
		initialBuy: %.8f
		solAmount: %.8f
		bondingCurveKey: %s
		vTokensInBondingCurve: %.8f
		vSolInBondingCurve: %.8f
		marketCapSol: %.8f
		name: %s
		symbol: %s
		uri: %s
		pool: %s
		====================
`,
		event.Signature,
		event.Mint,
		event.TraderPublicKey,
		event.TxType,
		event.InitialBuy,
		event.SolAmount,
		event.BondingCurveKey,
		event.VTokensInBondingCurve,
		event.VSolInBondingCurve,
		event.MarketCapSol,
		event.Name,
		event.Symbol,
		event.Uri,
		event.Pool,
	)
}
