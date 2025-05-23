package common

import "github.com/gagliardetto/solana-go"

type PumpfunBuyInstruction struct {
	Global                 solana.PublicKey
	FeeReceipt             solana.PublicKey
	Mint                   solana.PublicKey
	BondingCurve           solana.PublicKey
	AssociatedBondingCurve solana.PublicKey
	AssociatedUser         solana.PublicKey
	User                   solana.PublicKey
	SystemProgram          solana.PublicKey
	TokenProgram           solana.PublicKey
	CreatorVault           solana.PublicKey
	EventAuthority         solana.PublicKey
	Program                solana.PublicKey
	Input                  *BuyInstruction
}

// BuyInstruction 定义购买指令的输入数据
type BuyInstruction struct {
	Amount     uint64
	MaxSolCost uint64
}
