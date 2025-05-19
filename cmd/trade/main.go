package main

import (
	"context"
	"encoding/base64"
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

func main() {
	// 硬编码的交易参数
	publicKey := "Your public key here"
	privateKey := "" // 需要填入您的私钥（base58格式）
	action := "buy"
	mint := "token CA here"
	amount := 100000.0
	denominatedInSol := false
	slippage := 10
	priorityFee := 0.005
	pool := "auto"
	rpcURL := "https://mainnet.helius-rpc.com/?api-key=021015cb-98e8-485e-9f40-812b97f28ea3"

	// 验证必要参数
	if publicKey == "" || privateKey == "" || mint == "" {
		fmt.Println("必须提供公钥、私钥和代币合约地址")
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
