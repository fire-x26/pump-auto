package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
)

var (
	PRIVATE_KEY string
	RPC_URL     string
)

func init() {
	// 从环境变量读取配置
	PRIVATE_KEY = os.Getenv("SOLANA_PRIVATE_KEY")
	if PRIVATE_KEY == "" {
		log.Fatal("环境变量 SOLANA_PRIVATE_KEY 未设置")
	}

	RPC_URL = os.Getenv("SOLANA_RPC_URL")
	if RPC_URL == "" {
		log.Fatal("环境变量 SOLANA_RPC_URL 未设置")
	}
}

type TradeAction string

const (
	BUY  TradeAction = "buy"
	SELL TradeAction = "sell"
)

type PoolType string

const (
	AUTO         PoolType = "auto"
	PUMP         PoolType = "pump"
	RAYDIUM      PoolType = "raydium"
	PUMP_AMM     PoolType = "pump-amm"
	LAUNCHLAB    PoolType = "launchlab"
	RAYDIUM_CPMM PoolType = "raydium-cpmm"
	BONK         PoolType = "bonk"
)

// params
type TradeRequest struct {
	PublicKey        string      `json:"publicKey"`
	Action           TradeAction `json:"action"`
	Mint             string      `json:"mint"`
	Amount           float64     `json:"amount"`
	DenominatedInSol string      `json:"denominatedInSol"`
	Slippage         int         `json:"slippage"`
	PriorityFee      float64     `json:"priorityFee"`
	Pool             PoolType    `json:"pool"`
}

func ExecuteTrade(action TradeAction, mint string, amount float64, denominatedInSol bool, slippage int, priorityFee float64, pool PoolType) (string, error) {
	// 解析私钥（只解析一次）
	privateKey, err := solana.PrivateKeyFromBase58(PRIVATE_KEY)
	if err != nil {
		return "", fmt.Errorf("解析私钥失败: %v", err)
	}
	publicKey := privateKey.PublicKey()

	// 准备交易请求
	request := &TradeRequest{
		PublicKey:        publicKey.String(),
		Action:           action,
		Mint:             mint,
		Amount:           amount,
		DenominatedInSol: fmt.Sprintf("%t", denominatedInSol),
		Slippage:         slippage,
		PriorityFee:      priorityFee,
		Pool:             pool,
	}

	// 获取未签名交易
	data := url.Values{}
	data.Set("publicKey", request.PublicKey)
	data.Set("action", string(request.Action))
	data.Set("mint", request.Mint)
	data.Set("amount", fmt.Sprintf("%f", request.Amount))
	data.Set("denominatedInSol", request.DenominatedInSol)
	data.Set("slippage", fmt.Sprintf("%d", request.Slippage))
	data.Set("priorityFee", fmt.Sprintf("%f", request.PriorityFee))
	data.Set("pool", string(pool))

	resp, err := http.Post(
		"https://pumpportal.fun/api/trade-local",
		"application/x-www-form-urlencoded",
		strings.NewReader(data.Encode()),
	)
	if err != nil {
		return "", fmt.Errorf("发送交易请求失败: %v", err)
	}
	defer resp.Body.Close()

	transactionBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("读取响应失败: %v", err)
	}

	// 解析并签名交易
	tx, err := solana.TransactionFromBytes(transactionBytes)
	if err != nil {
		return "", fmt.Errorf("解析交易失败: %v", err)
	}

	// 获取最新区块哈希
	client := rpc.New(RPC_URL)
	recent, err := client.GetLatestBlockhash(context.Background(), rpc.CommitmentFinalized)
	if err != nil {
		return "", fmt.Errorf("获取区块哈希失败: %v", err)
	}

	// 更新区块哈希
	tx.Message.RecentBlockhash = recent.Value.Blockhash

	// 签名交易
	_, err = tx.Sign(
		func(pubkey solana.PublicKey) *solana.PrivateKey {
			if pubkey.Equals(publicKey) {
				return &privateKey
			}
			return nil
		},
	)
	if err != nil {
		return "", fmt.Errorf("签名交易失败: %v", err)
	}

	// 发送交易
	txSign, err := client.SendTransaction(context.Background(), tx)
	if err != nil {
		return "", fmt.Errorf("发送交易失败: %v", err)
	}

	log.Printf("交易发送成功: https://solscan.io/tx/%s", txSign.String())
	return txSign.String(), nil
}

func BuyToken(mint string, amount float64, denominatedInSol bool, slippage int, priorityFee float64, pool PoolType) (string, error) {
	return ExecuteTrade(BUY, mint, amount, denominatedInSol, slippage, priorityFee, pool)
}

func main() {
	mint := "FNvBEEWXmEQJG4Kg7UEg3LpXLDTkE8h7Z4fywTaJpump"
	amount := 0.001
	denominatedInSol := true
	slippage := 10
	priorityFee := 0.0005
	pool := PUMP

	txSignature, err := BuyToken(mint, amount, denominatedInSol, slippage, priorityFee, pool)
	if err != nil {
		log.Printf("交易失败: %v", err)
		return
	}
	log.Printf("交易成功: https://solscan.io/tx/%s", txSignature)
}
