package chainTx

import (
	"encoding/json"
	"fmt"
	"log"
	"testing"
)

// API返回的交易信息结构
type TransactionResponse struct {
	Signature string `json:"signature"`
	// 其他可能的字段...
}

func TestGetUnsignedTransaction(t *testing.T) {
	// 测试获取未签名交易
	publicKey, err := GetPublicKeyFromPrivateKey(PRIVATE_KEY)
	if err != nil {
		t.Fatalf("获取公钥失败: %v", err)
	}
	t.Logf("使用公钥: %s", publicKey)

	// 准备交易请求
	request := &TradeRequest{
		PublicKey:        publicKey,
		Action:           BUY,
		Mint:             "91WNez8D22NwBssQbkzjy4s2ipFrzpmn5hfvWVe2aY5p", // 示例代币地址
		Amount:           100000,
		DenominatedInSol: "false",
		Slippage:         10,
		PriorityFee:      0.005,
		Pool:             AUTO,
	}

	// 获取未签名交易
	unsignedTx, err := GetUnsignedTransaction(request)
	if err != nil {
		t.Fatalf("获取未签名交易失败: %v", err)
	}

	// 检查交易字节是否非空
	if len(unsignedTx) == 0 {
		t.Fatal("交易字节不应为空")
	}

	t.Logf("成功获取未签名交易，长度: %d bytes", len(unsignedTx))
}

func TestSignTransaction(t *testing.T) {
	// 测试签名交易
	publicKey, err := GetPublicKeyFromPrivateKey(PRIVATE_KEY)
	if err != nil {
		t.Fatalf("获取公钥失败: %v", err)
	}

	// 准备交易请求
	request := &TradeRequest{
		PublicKey:        publicKey,
		Action:           BUY,
		Mint:             "91WNez8D22NwBssQbkzjy4s2ipFrzpmn5hfvWVe2aY5p", // 示例代币地址
		Amount:           100000,
		DenominatedInSol: "false",
		Slippage:         10,
		PriorityFee:      0.005,
		Pool:             AUTO,
	}

	// 获取未签名交易
	unsignedTx, err := GetUnsignedTransaction(request)
	if err != nil {
		t.Fatalf("获取未签名交易失败: %v", err)
	}

	// 签名交易
	signedTx, err := SignTransaction(unsignedTx, PRIVATE_KEY)
	if err != nil {
		t.Fatalf("签名交易失败: %v", err)
	}

	// 检查签名后的交易字节是否非空
	if len(signedTx) == 0 {
		t.Fatal("签名后的交易字节不应为空")
	}

	t.Logf("成功签名交易，长度: %d bytes", len(signedTx))
}

// 完整的交易流程测试 (注意：此函数会实际执行交易！)
func TestFullTransactionFlow(t *testing.T) {
	// 跳过实际执行交易的测试，除非明确指定
	t.Skip("跳过实际执行交易的测试，移除此行以执行完整测试")

	mint := "91WNez8D22NwBssQbkzjy4s2ipFrzpmn5hfvWVe2aY5p" // 示例代币地址
	amount := 100000.0                                     // 代币数量
	denominatedInSol := false                              // false表示代币数量，true表示SOL数量
	slippage := 10                                         // 允许10%滑点
	priorityFee := 0.005                                   // 优先费
	pool := AUTO                                           // 自动选择交易池

	// 执行买入交易
	txSignature, err := BuyToken(mint, amount, denominatedInSol, slippage, priorityFee, pool)
	if err != nil {
		t.Fatalf("买入交易执行失败: %v", err)
	}

	t.Logf("交易成功! 交易签名: %s", txSignature)
	t.Logf("查看交易: https://solscan.io/tx/%s", txSignature)
}

// 手动测试函数，直接运行不作为单元测试
func TestManualTransaction() {
	log.Println("开始手动交易测试...")

	// 获取公钥
	publicKey, err := GetPublicKeyFromPrivateKey(PRIVATE_KEY)
	if err != nil {
		log.Fatalf("获取公钥失败: %v", err)
	}
	log.Printf("使用公钥: %s", publicKey)

	// 准备交易请求
	request := &TradeRequest{
		PublicKey:        publicKey,
		Action:           BUY,
		Mint:             "91WNez8D22NwBssQbkzjy4s2ipFrzpmn5hfvWVe2aY5p", // 示例代币地址
		Amount:           100000,
		DenominatedInSol: "false",
		Slippage:         10,
		PriorityFee:      0.005,
		Pool:             AUTO,
	}

	// 获取未签名交易
	log.Println("正在获取未签名交易...")
	unsignedTx, err := GetUnsignedTransaction(request)
	if err != nil {
		log.Fatalf("获取未签名交易失败: %v", err)
	}
	log.Printf("成功获取未签名交易，长度: %d bytes", len(unsignedTx))

	// 签名交易
	log.Println("正在签名交易...")
	signedTx, err := SignTransaction(unsignedTx, PRIVATE_KEY)
	if err != nil {
		log.Fatalf("签名交易失败: %v", err)
	}
	log.Printf("成功签名交易，长度: %d bytes", len(signedTx))

	// 发送交易（需要手动确认）
	var confirm string
	fmt.Printf("是否发送交易? [y/N]: ")
	fmt.Scanln(&confirm)
	if confirm != "y" && confirm != "Y" {
		log.Println("已取消交易")
		return
	}

	// 发送签名后的交易
	log.Println("正在发送交易...")
	txSignature, err := SendSignedTransaction(signedTx)
	if err != nil {
		log.Fatalf("发送交易失败: %v", err)
	}

	// 输出交易结果
	log.Printf("交易成功! 交易签名: %s", txSignature)
	log.Printf("查看交易: https://solscan.io/tx/%s", txSignature)

	// 尝试解析返回结果
	var response TransactionResponse
	if err := json.Unmarshal([]byte(txSignature), &response); err != nil {
		// 如果不是JSON格式，直接使用返回的签名字符串
		log.Printf("直接返回的签名: %s", txSignature)
	} else {
		// 如果是JSON格式，提取签名
		log.Printf("JSON响应中的签名: %s", response.Signature)
	}
}

// 主函数，用于手动运行测试
func ExampleTransaction() {
	TestManualTransaction()
	// Output:
	// 根据实际运行情况输出
}
