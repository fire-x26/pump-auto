package chainTx

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"net/url"
	"pump_auto/internal/common"
	"strconv"
	"strings"
	"time"

	"github.com/davecgh/go-spew/spew"
	bin "github.com/gagliardetto/binary"

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
	if sellPercent == "100%" && denominatedInSol == false {
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

	// 打印请求数据
	log.Printf("发送交易请求数据:")
	log.Printf("--------------------------------")

	log.Printf("URL: https://pumpportal.fun/api/trade-local")
	log.Printf("--------------------------------")

	log.Printf("请求参数:")
	for key, values := range data {
		log.Printf("%s: %v", key, values)
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

	// // 添加详细日志
	// log.Printf("交易详情:")
	// log.Printf("- 交易指令数量: %d", len(tx.Message.Instructions))
	// log.Printf("- 交易账户数量: %d", len(tx.Message.AccountKeys))

	// // 首先打印所有账户
	// log.Printf("交易账户列表:")
	// for i, acc := range tx.Message.AccountKeys {
	// 	log.Printf("- 账户 %d: %s", i, acc.String())
	// }

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

	// 设置轮询参数
	maxRetries := 15 // 最多重试15次
	retryInterval := 2 * time.Second

	var sign string
	var err error

	// 开始重试循环
	for i := 0; i < maxRetries; i++ {
		sign, err = ExecuteTrade(common.BUY, mint, amount, "", true, slippage, priorityFee, pool)
		if err == nil {
			return sign, nil // 如果成功，直接返回
		}

		log.Printf("第 %d 次购买代币 %s 失败: %v，等待2秒后重试...", i+1, mint, err)
		time.Sleep(retryInterval)
	}

	return "", fmt.Errorf("购买代币 %s 失败，已达到最大重试次数: %v", mint, err)
}

func SellToken(mint string, amount float64, sellPercent string, denominatedInSol bool, slippage int, priorityFee float64, pool common.PoolType) (string, error) {
	// 设置轮询参数
	maxRetries := 15 // 最多重试15次
	retryInterval := 2 * time.Second

	var sign string
	var err error

	// 开始重试循环
	for i := 0; i < maxRetries; i++ {
		sign, err = ExecuteTrade(common.SELL, mint, amount, sellPercent, false, slippage, priorityFee, pool)
		if err == nil {
			return sign, nil // 如果成功，直接返回
		}

		log.Printf("第 %d 次出售代币 %s 失败: %v，等待2秒后重试...", i+1, mint, err)
		time.Sleep(retryInterval)
	}

	return "", fmt.Errorf("出售代币 %s 失败，已达到最大重试次数: %v", mint, err)
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

	// 设置轮询参数
	maxRetries := 15 // 最多等待30秒
	retryInterval := 1 * time.Second

	for i := 0; i < maxRetries; i++ {
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
			log.Printf("第 %d 次获取代币账户失败: %v，等待1秒后重试...", i+1, err)
			time.Sleep(retryInterval)
			continue
		}

		if len(tokenAccounts.Value) == 0 {
			log.Printf("第 %d 次检查：用户没有代币 %s 的账户，等待1秒后重试...", i+1, mint)
			time.Sleep(retryInterval)
			continue
		}

		tokenVault := solana.MustPublicKeyFromBase58(tokenAccounts.Value[0].Pubkey.String())
		// 获取代币账户余额
		balance, err := client.GetTokenAccountBalance(
			context.Background(),
			tokenVault,
			rpc.CommitmentFinalized,
		)
		if err != nil {
			log.Printf("第 %d 次获取代币余额失败: %v，等待1秒后重试...", i+1, err)
			time.Sleep(retryInterval)
			continue
		}

		// 将余额字符串转换为 float64
		result, err := strconv.ParseFloat(balance.Value.Amount, 64)
		if err != nil {
			log.Printf("第 %d 次转换余额失败: %v，等待1秒后重试...", i+1, err)
			time.Sleep(retryInterval)
			continue
		}

		// 除以代币的decimal（6）
		result = result / math.Pow10(6)

		log.Printf("代币 %s 余额: %f", mint, result)
		return result, nil
	}

	return 0, fmt.Errorf("获取代币 %s 余额超时，已重试 %d 次", mint, maxRetries)
}
func ParseTxSign(txSig solana.Signature) (float64, error) {
	client := rpc.New(RPC_URL)
	var maxVersion uint64 = 0

	// 设置轮询参数
	maxRetries := 20 // 60秒 / 3秒 = 20次
	retryInterval := 3 * time.Second

	var out *rpc.GetTransactionResult
	var err error

	// 开始轮询查询
	for i := 0; i < maxRetries; i++ {
		out, err = client.GetTransaction(
			context.Background(),
			txSig,
			&rpc.GetTransactionOpts{
				Encoding:                       solana.EncodingBase64,
				MaxSupportedTransactionVersion: &maxVersion,
			},
		)
		if err == nil {
			break // 如果查询成功，跳出循环
		}

		fmt.Printf("第 %d 次查询交易失败: %v，等待3秒后重试...\n", i+1, err)
		time.Sleep(retryInterval)
	}

	if err != nil {
		fmt.Printf("查询交易失败，已达到最大重试次数: %v\n", err)
		return 0, err
	}

	// 解码交易
	decodedTx, err := solana.TransactionFromDecoder(bin.NewBinDecoder(out.Transaction.GetBinary()))
	if err != nil {
		fmt.Printf("解码交易失败: %v\n", err)
		return 0, err
	}

	// 获取交易元数据
	meta, err := client.GetTransaction(
		context.Background(),
		txSig,
		&rpc.GetTransactionOpts{
			Encoding:                       solana.EncodingBase64,
			MaxSupportedTransactionVersion: &maxVersion,
		},
	)
	if err != nil {
		fmt.Printf("获取交易元数据失败: %v\n", err)
		return 0, err
	}

	// 检查元数据
	if meta.Meta == nil {
		fmt.Println("交易元数据为空")
		return 0, err
	}

	// 查找第4条指令的内嵌指令（索引 3）
	var innerInstructions []solana.CompiledInstruction
	for _, inner := range meta.Meta.InnerInstructions {
		if inner.Index == 3 { // 第4条指令的内嵌指令
			innerInstructions = inner.Instructions
			break
		}
	}

	if len(innerInstructions) == 0 {
		fmt.Println("第4条指令没有内嵌指令")
		return 0, err
	}

	if len(innerInstructions) < 1 {
		fmt.Printf("内嵌指令数量不足，期望至少 1 条，实际 %d 条\n", len(innerInstructions))
		return 0, err
	}

	// 提取第1条内嵌指令（索引 0）
	targetInnerInstruction := innerInstructions[0]
	fmt.Println("第4条指令的第1条内嵌指令:")
	spew.Dump(targetInnerInstruction)

	// 检查账户数量
	if len(targetInnerInstruction.Accounts) < 5 {
		fmt.Printf("内嵌指令账户数量不足，期望至少 5 个，实际 %d 个\n", len(targetInnerInstruction.Accounts))
		return 0, err
	}
	// 映射账户到 PumpfunBuyInstruction
	pumpfunInstruction := common.PumpfunBuyInstruction{
		Mint: decodedTx.Message.AccountKeys[targetInnerInstruction.Accounts[2]],
		User: decodedTx.Message.AccountKeys[targetInnerInstruction.Accounts[6]],
	}

	// 解析指令数据
	data := targetInnerInstruction.Data
	if len(data) < 16 { // 确保数据长度足够
		fmt.Printf("指令数据长度不足，期望至少 16 字节，实际 %d 字节\n", len(data))
		return 0, err
	}

	// 解析 AmountOut
	buyInstruction := common.BuyInstruction{
		Amount:     binary.LittleEndian.Uint64(data[8:16]),
		MaxSolCost: binary.LittleEndian.Uint64(data[16:24]),
	}
	pumpfunInstruction.Input = &buyInstruction

	// 打印解析结果
	fmt.Println("解析的 PumpfunBuyInstruction（内嵌指令）:")
	spew.Dump(pumpfunInstruction)

	// 格式化输出
	fmt.Printf("Token: %s\n", pumpfunInstruction.Mint)
	fmt.Printf("User Wallet: %s\n", pumpfunInstruction.User)
	fmt.Printf("AmountOut: %d (%.6f tokens, 精度 6)\n", buyInstruction.Amount, float64(buyInstruction.Amount)/1_000_000)
	return float64(buyInstruction.Amount) / 1_000_000, nil
}
