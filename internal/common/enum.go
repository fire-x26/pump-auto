package common

type TradeAction string

type PoolType string

const (
	BUY  TradeAction = "buy"
	SELL TradeAction = "sell"
)
const (
	AUTO         PoolType = "auto"
	PUMP         PoolType = "pump"
	RAYDIUM      PoolType = "raydium"
	PUMP_AMM     PoolType = "pump-amm"
	LAUNCHLAB    PoolType = "launchlab"
	RAYDIUM_CPMM PoolType = "raydium-cpmm"
	BONK         PoolType = "bonk"
)

const MAX_HOLD_TOKEN = 5
const PRECISION = 16
