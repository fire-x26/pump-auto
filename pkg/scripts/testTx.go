package main

import (
	"bytes"
	"context"
	_ "context"
	"encoding/json"
	"fmt"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"io"
	"net/http"

	_ "github.com/gagliardetto/solana-go/rpc"
	_ "github.com/gagliardetto/solana-go/rpc/jsonrpc"
)

// TradeRequest represents the request body for the pumpportal API
type TradeRequest struct {
	PublicKey        string  `json:"publicKey"`
	Action           string  `json:"action"`
	Mint             string  `json:"mint"`
	Amount           string  `json:"amount"` // Changed to string to support percentages
	DenominatedInSol string  `json:"denominatedInSol"`
	Slippage         int     `json:"slippage"`
	PriorityFee      float64 `json:"priorityFee"`
	Pool             string  `json:"pool,omitempty"` // Optional, defaults to "pump"
}

func main() {
	// Initialize Solana RPC client
	rpcEndpoint := "https://mainnet.helius-rpc.com/?api-key=021015cb-98e8-485e-9f40-812b97f28ea3"
	client := rpc.New(rpcEndpoint)

	// Prepare trade request
	tradeReq := TradeRequest{
		PublicKey:        "EHKyF2UVTtrE9ZJpDsStkQP52LzkY52qFJxbZgJhoppp",
		Action:           "buy",                                          // "buy" or "sell"
		Mint:             "5BvJsQ7UyWJMm2pmQeNjYLFsLaRuyyQcaBwFTPhupump", // contract address of the token
		Amount:           "100000",                                       // Use string: number (e.g., "100000") or percentage (e.g., "100%")
		DenominatedInSol: "false",                                        // "true" for SOL, "false" for tokens
		Slippage:         10,                                             // percent slippage
		PriorityFee:      0.005,                                          // priority fee
		Pool:             "auto",                                         // exchange: "pump", "raydium", etc. (optional)
	}

	// Convert request to JSON
	reqBody, err := json.Marshal(tradeReq)
	if err != nil {
		fmt.Printf("Error marshaling request: %v\n", err)
		return
	}

	// Make POST request to pumpportal
	resp, err := http.Post(
		"https://pumpportal.fun/api/trade-local",
		"application/json",
		bytes.NewBuffer(reqBody),
	)
	if err != nil {
		fmt.Printf("Error making trade request: %v\n", err)
		return
	}
	defer resp.Body.Close()

	// Check HTTP status
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		fmt.Printf("API error: status %d, response: %s\n", resp.StatusCode, string(body))
		return
	} else {
		fmt.Println("相应获取成功")
	}
	txBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("Error reading response: %v\n", err)
		return
	}

	privateKey := "3sXbWaTC7dqsg8Ck35T5XZketcTVpRCaFVWWafv5T9ES3NBXWNutBRmHUpiGri4H59nFmK4v25v8SKK2v3Z7hGSU"
	account, err := solana.PrivateKeyFromBase58(privateKey)
	if err != nil {
		fmt.Printf("Error loading private key: %v\n", err)
		return
	}

	// Deserialize transaction

	//tsa, err := solana.NewTransaction()
	// 6. 反序列化交易
	var tx *solana.Transaction
	// 尝试作为二进制交易数据处理
	tx, err = solana.TransactionFromBytes(txBytes)
	if err != nil {
		// 如果二进制失败，尝试作为 Base64 处理
		fmt.Printf("TransactionFromBytes failed: %v\n", err)
		tx, err = solana.TransactionFromBase64(string(txBytes))
		if err != nil {
			fmt.Printf("TransactionFromBase64 failed: %v\n", err)
			return
		}
	}
	// Sign transaction
	_, err = tx.Sign(func(key solana.PublicKey) *solana.PrivateKey {
		if key.Equals(account.PublicKey()) {
			return &account
		}
		return nil
	})
	if err != nil {
		fmt.Printf("Error signing transaction: %v\n", err)
		return
	}
	maxRetries := uint(3) // Create a uint variable

	// Send transaction
	ctx := context.Background()
	txSig, err := client.SendTransactionWithOpts(
		ctx,
		tx,
		rpc.TransactionOpts{
			SkipPreflight:       false,
			PreflightCommitment: rpc.CommitmentConfirmed,
			MaxRetries:          &maxRetries, // Add retries for RPC failures
		},
	)
	if err != nil {
		fmt.Printf("Error sending transaction: %v\n", err)
		return
	}

	// Print transaction link
	fmt.Printf("Transaction: https://solscan.io/tx/%s\n", txSig)
}
