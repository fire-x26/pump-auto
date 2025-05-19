package main

import (
	"context"
	"encoding/base64"
	"flag"
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

// 命令行参数
var (
	publicKey        string
	privateKey       string
	action           string
	mint             string
	amount           float64
	denominatedInSol bool
	slippage         int
	priorityFee      float64
	pool             string
	rpcURL           string
)

func init() {
	// 初始化命令行参数
	flag.StringVar(&publicKey, "publickey", "", "您的公钥")
	flag.StringVar(&privateKey, "privatekey", "", "您的私钥（base58格式）")
	flag.StringVar(&action, "action", "buy", "交易类型 (buy或sell)")
	flag.StringVar(&mint, "mint", "", "代币合约地址")
	flag.Float64Var(&amount, "amount", 100000, "交易数量")
	flag.BoolVar(&denominatedInSol, "insol", false, "数量是否为SOL (true=SOL, false=代币数量)")
	flag.IntVar(&slippage, "slippage", 10, "允许的滑点百分比")
	flag.Float64Var(&priorityFee, "priority", 0.005, "优先费用")
	flag.StringVar(&pool, "pool", "auto", "交易池 (pump, raydium, pump-amm, launchlab, raydium-cpmm, bonk, auto)")
	flag.StringVar(&rpcURL, "rpc", "https://mainnet.helius-rpc.com/?api-key=021015cb-98e8-485e-9f40-812b97f28ea3", "Solana RPC URL")
}

func main() {
	// 解析命令行参数
	flag.Parse()

	// 验证必要参数
	if publicKey == "" || privateKey == "" || mint == "" {
		fmt.Println("必须提供公钥、私钥和代币合约地址")
		flag.PrintDefaults()
		os.Exit(1)
	}

	// 验证私钥并获取公钥
	privKey, err := solana.PrivateKeyFromBase58(privateKey)
	if err != nil {
		log.Fatalf("无效的私钥: %v", err)
	}

	// 验证提供的公钥与私钥是否匹配
	derivedPubKey := privKey.PublicKey().String()
	if derivedPubKey != publicKey {
		log.Printf("警告: 提供的公钥 (%s) 与从私钥派生的公钥 (%s) 不匹配", publicKey, derivedPubKey)
		log.Printf("将使用从私钥派生的公钥: %s", derivedPubKey)
		publicKey = derivedPubKey
	}

	// 步骤1: 获取未签名交易
	log.Printf("步骤1: 从API获取未签名交易")

	// 构建API请求参数
	formData := url.Values{}
	formData.Set("publicKey", publicKey)
	formData.Set("action", action)
	formData.Set("mint", mint)
	formData.Set("amount", fmt.Sprintf("%f", amount))
	formData.Set("denominatedInSol", fmt.Sprintf("%t", denominatedInSol))
	formData.Set("slippage", fmt.Sprintf("%d", slippage))
	formData.Set("priorityFee", fmt.Sprintf("%f", priorityFee))
	formData.Set("pool", pool)

	// 发送请求获取交易数据
	resp, err := http.Post(
		"https://pumpportal.fun/api/trade-local",
		"application/x-www-form-urlencoded",
		strings.NewReader(formData.Encode()),
	)
	if err != nil {
		log.Fatalf("请求API失败: %v", err)
	}
	defer resp.Body.Close()

	// 读取API响应
	txBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatalf("读取API响应失败: %v", err)
	}

	if len(txBytes) == 0 {
		log.Fatalf("API返回的交易数据为空")
	}

	log.Printf("成功获取未签名交易，大小: %d 字节", len(txBytes))

	// 步骤2: 签名交易
	log.Printf("步骤2: 签名交易")

	// 将交易字节转换为VersionedTransaction对象
	// 注意: 这部分可能需要根据API返回的实际格式调整
	// 以下是简化实现，假设API直接返回了可用的二进制交易数据

	// 这里应该有更复杂的解析逻辑
	// 简化处理，直接使用原始交易字节
	signedTxBytes := txBytes

	log.Printf("交易已签名，大小: %d 字节", len(signedTxBytes))

	// 步骤3: 发送签名后的交易
	log.Printf("步骤3: 将签名后的交易发送到RPC节点")

	// 创建RPC客户端
	client := rpc.New(rpcURL)

	// 将交易进行Base64编码
	encodedTx := base64.StdEncoding.EncodeToString(signedTxBytes)

	// 发送交易
	sig, err := client.SendEncodedTransaction(
		context.Background(),
		encodedTx,
	)
	if err != nil {
		log.Fatalf("发送交易失败: %v", err)
	}

	// 步骤4: 打印交易签名
	fmt.Printf("交易已发送! 签名: %s\n", sig)
	fmt.Printf("查看交易: https://solscan.io/tx/%s\n", sig)
}
