package common

type TradeReq struct {
	PublicKey        string      `json:"publicKey"`
	Action           TradeAction `json:"action"`
	Mint             string      `json:"mint"`
	Amount           float64     `json:"amount"`
	DenominatedInSol bool        `json:"denominatedInSol"`
	Slippage         int         `json:"slippage"`
	PriorityFee      float64     `json:"priorityFee"`
	Pool             PoolType    `json:"pool"`
}
