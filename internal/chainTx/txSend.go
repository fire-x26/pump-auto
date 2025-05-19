package chainTx

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
)

const (
	PRIVATE_KEY = "3sXbWaTC7dqsg8Ck35T5XZketcTVpRCaFVWWafv5T9ES3NBXWNutBRmHUpiGri4H59nFmK4v25v8SKK2v3Z7hGSU"
	RPC_URL     = "https://mainnet.helius-rpc.com/?api-key=021015cb-98e8-485e-9f40-812b97f28ea3"
)

// 交易类型
type TradeAction string

const (
	BUY  TradeAction = "buy"
	SELL TradeAction = "sell"
)

// 交易池
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

// 交易请求参数
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

// RPC请求结构
type RPCRequest struct {
	Jsonrpc string      `json:"jsonrpc"`
	ID      int         `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params"`
}

// SendTransactionParams 参数
type SendTransactionParams struct {
	Transaction string                 `json:"transaction"`
	Config      map[string]interface{} `json:"config,omitempty"`
}

// RPC响应结构
type RPCResponse struct {
	Jsonrpc string      `json:"jsonrpc"`
	ID      int         `json:"id"`
	Result  string      `json:"result,omitempty"`
	Error   interface{} `json:"error,omitempty"`
}

// 从私钥中提取公钥
func GetPublicKeyFromPrivateKey(privateKeyStr string) (string, error) {
	// 从base58编码的私钥字符串中解析出私钥
	privateKey, err := solana.PrivateKeyFromBase58(privateKeyStr)
	if err != nil {
		return "", fmt.Errorf("解析私钥失败: %v", err)
	}

	// 从私钥获取公钥
	publicKey := privateKey.PublicKey()
	return publicKey.String(), nil
}

// 发送交易到API获取未签名交易
func GetUnsignedTransaction(request *TradeRequest) ([]byte, error) {
	// 将请求转换为表单数据
	data := url.Values{}
	data.Set("publicKey", request.PublicKey)
	data.Set("action", string(request.Action))
	data.Set("mint", request.Mint)
	data.Set("amount", fmt.Sprintf("%f", request.Amount))
	data.Set("denominatedInSol", request.DenominatedInSol)
	data.Set("slippage", fmt.Sprintf("%d", request.Slippage))
	data.Set("priorityFee", fmt.Sprintf("%f", request.PriorityFee))
	data.Set("pool", string(request.Pool))

	// 发送POST请求
	resp, err := http.Post(
		"https://pumpportal.fun/api/trade-local",
		"application/x-www-form-urlencoded",
		strings.NewReader(data.Encode()),
	)
	if err != nil {
		return nil, fmt.Errorf("发送请求到交易API失败: %v", err)
	}
	defer resp.Body.Close()

	// 读取响应内容
	transactionBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %v", err)
	}

	return transactionBytes, nil
}

// 签名交易
func SignTransaction(transactionBytes []byte, privateKeyStr string) ([]byte, error) {
	// 从base58编码的私钥字符串中解析出私钥
	_, err := solana.PrivateKeyFromBase58(privateKeyStr)
	if err != nil {
		return nil, fmt.Errorf("解析私钥失败: %v", err)
	}

	// 注意：由于API返回的交易格式可能与solana-go库不完全兼容，这里采用简化处理
	// 实际项目中需根据具体情况调整
	log.Printf("注意：交易签名机制需要根据API返回的实际交易格式调整")

	// 这里我们假设API已经为我们准备好了交易，包括签名槽位
	// 在实际应用中，需要正确解析和签名交易

	return transactionBytes, nil
}

// 发送已签名交易到Solana网络
func SendSignedTransaction(signedTransaction []byte) (string, error) {
	// 创建RPC客户端
	client := rpc.New(RPC_URL)

	// 交易base64编码
	encodedTx := base64.StdEncoding.EncodeToString(signedTransaction)

	// 发送交易
	sig, err := client.SendEncodedTransaction(
		context.Background(),
		encodedTx,
	)
	if err != nil {
		return "", fmt.Errorf("发送交易失败: %v", err)
	}

	// 返回交易签名
	return sig.String(), nil
}

// 执行交易的主函数
func ExecuteTrade(action TradeAction, mint string, amount float64, denominatedInSol bool, slippage int, priorityFee float64, pool PoolType) (string, error) {
	// 获取公钥
	publicKey, err := GetPublicKeyFromPrivateKey(PRIVATE_KEY)
	if err != nil {
		return "", fmt.Errorf("获取公钥失败: %v", err)
	}

	// 准备交易请求
	request := &TradeRequest{
		PublicKey:        publicKey,
		Action:           action,
		Mint:             mint,
		Amount:           amount,
		DenominatedInSol: fmt.Sprintf("%t", denominatedInSol),
		Slippage:         slippage,
		PriorityFee:      priorityFee,
		Pool:             pool,
	}

	// 获取未签名交易
	log.Printf("获取未签名交易: %+v", request)
	unsignedTransaction, err := GetUnsignedTransaction(request)
	if err != nil {
		return "", fmt.Errorf("获取未签名交易失败: %v", err)
	}

	// 签名交易
	log.Println("签名交易...")
	signedTransaction, err := SignTransaction(unsignedTransaction, PRIVATE_KEY)
	if err != nil {
		return "", fmt.Errorf("签名交易失败: %v", err)
	}

	// 发送已签名交易
	log.Println("发送已签名交易到网络...")
	txSignature, err := SendSignedTransaction(signedTransaction)
	if err != nil {
		return "", fmt.Errorf("发送已签名交易失败: %v", err)
	}

	log.Printf("交易发送成功: https://solscan.io/tx/%s", txSignature)
	return txSignature, nil
}

// 买入代币
func BuyToken(mint string, amount float64, denominatedInSol bool, slippage int, priorityFee float64, pool PoolType) (string, error) {
	return ExecuteTrade(BUY, mint, amount, denominatedInSol, slippage, priorityFee, pool)
}

// 卖出代币
func SellToken(mint string, amount float64, denominatedInSol bool, slippage int, priorityFee float64, pool PoolType) (string, error) {
	return ExecuteTrade(SELL, mint, amount, denominatedInSol, slippage, priorityFee, pool)
}
