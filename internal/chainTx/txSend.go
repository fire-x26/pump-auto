package chainTx

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"pump_auto/internal/common"
	"strconv"
	"strings"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/programs/token"
	"github.com/gagliardetto/solana-go/rpc"
)

const PUBLIC_KEY = "5zUyGNwtCCyYthLqUzYEDhYhfRrK9eHNf4DQ4KhQEirm"

// params
type TradeRequest struct {
	PublicKey        string             `json:"publicKey"`
	Action           common.TradeAction `json:"action"`
	Mint             string             `json:"mint"`
	Amount           float64            `json:"amount"`
	DenominatedInSol string             `json:"denominatedInSol"`
	Slippage         int                `json:"slippage"`
	PriorityFee      float64            `json:"priorityFee"`
	Pool             common.PoolType    `json:"pool"`
}
type TradeRequestPercent struct {
	PublicKey        string             `json:"publicKey"`
	Action           common.TradeAction `json:"action"`
	Mint             string             `json:"mint"`
	Amount           string             `json:"amount"`
	DenominatedInSol string             `json:"denominatedInSol"`
	Slippage         int                `json:"slippage"`
	PriorityFee      float64            `json:"priorityFee"`
	Pool             common.PoolType    `json:"pool"`
}

func ExecuteTrade(action common.TradeAction, mint string, amount float64, sellPercent string, denominatedInSol bool, slippage int, priorityFee float64, pool common.PoolType) (string, error) {
	// 解析私钥（只解析一次）
	privateKey, err := solana.PrivateKeyFromBase58(PRIVATE_KEY)
	if err != nil {
		return "", fmt.Errorf("解析私钥失败: %v", err)
	}

	// 使用已定义的公钥
	publicKey, err := solana.PublicKeyFromBase58(PUBLIC_KEY)
	if err != nil {
		return "", fmt.Errorf("解析公钥失败: %v", err)
	}

	var request interface{}
	if sellPercent != "" && denominatedInSol == false {
		request = &TradeRequestPercent{
			PublicKey:        publicKey.String(),
			Action:           action,
			Mint:             mint,
			Amount:           sellPercent,
			DenominatedInSol: fmt.Sprintf("%t", denominatedInSol),
			Slippage:         slippage,
			PriorityFee:      priorityFee,
			Pool:             pool,
		}
	} else {
		request = &TradeRequest{
			PublicKey:        publicKey.String(),
			Action:           action,
			Mint:             mint,
			Amount:           amount,
			DenominatedInSol: fmt.Sprintf("%t", denominatedInSol),
			Slippage:         slippage,
			PriorityFee:      priorityFee,
			Pool:             pool,
		}
	}

	// 准备交易请求
	data := url.Values{}
	switch r := request.(type) {
	case *TradeRequest:
		data.Set("publicKey", r.PublicKey)
		data.Set("action", string(r.Action))
		data.Set("mint", r.Mint)
		data.Set("amount", fmt.Sprintf("%f", r.Amount))
		data.Set("denominatedInSol", r.DenominatedInSol)
		data.Set("slippage", fmt.Sprintf("%d", r.Slippage))
		data.Set("priorityFee", fmt.Sprintf("%f", r.PriorityFee))
		data.Set("pool", string(r.Pool))
	case *TradeRequestPercent:
		data.Set("publicKey", r.PublicKey)
		data.Set("action", string(r.Action))
		data.Set("mint", r.Mint)
		data.Set("amount", r.Amount)
		data.Set("denominatedInSol", r.DenominatedInSol)
		data.Set("slippage", fmt.Sprintf("%d", r.Slippage))
		data.Set("priorityFee", fmt.Sprintf("%f", r.PriorityFee))
		data.Set("pool", string(r.Pool))
	}

	resp, err := http.Post(
		"https://pumpportal.fun/api/trade-local",
		"application/x-www-form-urlencoded",
		strings.NewReader(data.Encode()),
	)
	if err != nil {
		return "", fmt.Errorf("发送交易请求失败: %v", err)
	}
	defer resp.Body.Close()

	// 添加响应状态码日志
	log.Printf("API响应状态码: %d", resp.StatusCode)

	transactionBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("读取响应失败: %v", err)
	}

	// 添加API响应日志
	log.Printf("API响应内容: %s", string(transactionBytes))

	// 解析并签名交易
	tx, err := solana.TransactionFromBytes(transactionBytes)
	if err != nil {
		return "", fmt.Errorf("解析交易失败: %v", err)
	}

	// 添加详细日志
	log.Printf("交易详情:")
	log.Printf("- 交易指令数量: %d", len(tx.Message.Instructions))
	log.Printf("- 交易账户数量: %d", len(tx.Message.AccountKeys))

	// 首先打印所有账户
	log.Printf("交易账户列表:")
	for i, acc := range tx.Message.AccountKeys {
		log.Printf("- 账户 %d: %s", i, acc.String())
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

func BuyToken(mint string, amount float64, denominatedInSol bool, slippage int, priorityFee float64, pool common.PoolType) (string, error) {
	fmt.Printf("mint:%s,amout:%f,pool:%s", mint, amount, pool)
	return ExecuteTrade(common.BUY, mint, amount, "", true, slippage, priorityFee, pool)
}
func SellToken(mint string, amount float64, sellPercent string, denominatedInSol bool, slippage int, priorityFee float64, pool common.PoolType) (string, error) {
	return ExecuteTrade(common.SELL, mint, amount, sellPercent, false, slippage, priorityFee, pool)
}

// GetTokenDecimal 获取代币的精度
func GetTokenDecimal(mint string) (uint8, error) {
	client := rpc.New(RPC_URL)

	// 将mint地址转换为PublicKey
	mintPubkey, err := solana.PublicKeyFromBase58(mint)
	if err != nil {
		return 0, fmt.Errorf("无效的代币地址: %v", err)
	}

	// 使用 GetAccountDataInto 直接获取代币信息
	var mintInfo token.Mint
	err = client.GetAccountDataInto(
		context.Background(),
		mintPubkey,
		&mintInfo,
	)
	if err != nil {
		return 0, fmt.Errorf("获取代币信息失败: %v", err)
	}

	// 打印代币信息用于调试
	log.Printf("代币信息: 精度=%d, 供应量=%d", mintInfo.Decimals, mintInfo.Supply)

	return mintInfo.Decimals, nil
}

// GetTokenBalance 获取用户对特定代币的余额
func GetTokenBalance(mint string) (float64, error) {
	client := rpc.New(RPC_URL)

	// 使用已定义的公钥
	publicKey, err := solana.PublicKeyFromBase58(PUBLIC_KEY)
	if err != nil {
		return 0, fmt.Errorf("解析公钥失败: %v", err)
	}

	// 将mint地址转换为PublicKey
	mintPubkey, err := solana.PublicKeyFromBase58(mint)
	if err != nil {
		return 0, fmt.Errorf("无效的代币地址: %v", err)
	}

	// 获取用户的代币账户
	tokenAccounts, err := client.GetTokenAccountsByOwner(
		context.Background(),
		publicKey,
		&rpc.GetTokenAccountsConfig{
			Mint: &mintPubkey,
		},
		&rpc.GetTokenAccountsOpts{
			Encoding: solana.EncodingBase64,
		},
	)
	if err != nil {
		return 0, fmt.Errorf("获取代币账户失败: %v", err)
	}

	if len(tokenAccounts.Value) == 0 {
		log.Printf("用户没有代币 %s 的账户", mint)
		return 0, nil // 用户没有该代币的账户
	}

	tokenVault := solana.MustPublicKeyFromBase58(tokenAccounts.Value[0].Pubkey.String())
	// 获取代币账户余额
	balance, err := client.GetTokenAccountBalance(
		context.Background(),
		tokenVault,
		rpc.CommitmentFinalized,
	)
	if err != nil {
		return 0, fmt.Errorf("获取代币余额失败: %v", err)
	}

	// 将余额字符串转换为 float64
	result, err := strconv.ParseFloat(balance.Value.Amount, 64)
	if err != nil {
		return 0, fmt.Errorf("转换余额失败: %v", err)
	}

	log.Printf("代币 %s 余额: %f", mint, result)
	return result, nil
}
