package chainTx

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"pump_auto/internal/common"
	"strings"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
)

const PRIVATE_KEY = ""
const RPC_URL = ""

//func init() {
//	// 获取当前工作目录
//	currentDir, err := os.Getwd()
//	if err != nil {
//		log.Printf("警告: 无法获取当前工作目录: %v", err)
//	} else {
//		log.Printf("当前工作目录: %s", currentDir)
//	}
//
//	// 尝试从多个位置加载环境变量
//	envPaths := []string{
//		".env",                                  // 当前目录
//		"../.env",                               // 上级目录
//		filepath.Join("..", ".env"),             // 使用filepath处理路径
//		filepath.Join(currentDir, ".env"),       // 使用完整路径
//		filepath.Join(currentDir, "..", ".env"), // 上级目录的完整路径
//	}
//
//	log.Printf("尝试加载环境变量文件，搜索路径:")
//	for _, path := range envPaths {
//		log.Printf("  - %s", path)
//		if _, err := os.Stat(path); err == nil {
//			log.Printf("找到环境变量文件: %s", path)
//			// 文件存在，尝试读取
//			content, err := os.ReadFile(path)
//			if err != nil {
//				log.Printf("警告: 无法读取环境变量文件 %s: %v", path, err)
//				continue
//			}
//
//			log.Printf("成功读取环境变量文件内容:")
//			lines := strings.Split(string(content), "\n")
//			for _, line := range lines {
//				line = strings.TrimSpace(line)
//				if line == "" || strings.HasPrefix(line, "#") {
//					continue
//				}
//
//				parts := strings.SplitN(line, "=", 2)
//				if len(parts) != 2 {
//					continue
//				}
//
//				key := strings.TrimSpace(parts[0])
//				value := strings.TrimSpace(parts[1])
//
//				// 移除值两端的引号
//				value = strings.Trim(value, "\"'")
//
//				// 打印环境变量（隐藏私钥）
//				if key == "SOLANA_PRIVATE_KEY" {
//					log.Printf("  %s=******", key)
//				} else {
//					log.Printf("  %s=%s", key, value)
//				}
//
//				// 设置环境变量
//				os.Setenv(key, value)
//			}
//			log.Printf("成功从 %s 加载环境变量", path)
//			break
//		} else {
//			log.Printf("未找到环境变量文件: %s", path)
//		}
//	}
//
//	// 从环境变量读取配置
//	PRIVATE_KEY = os.Getenv("SOLANA_PRIVATE_KEY")
//	if PRIVATE_KEY == "" {
//		log.Fatal("环境变量 SOLANA_PRIVATE_KEY 未设置")
//	}
//
//	RPC_URL = os.Getenv("SOLANA_RPC_URL")
//	if RPC_URL == "" {
//		log.Fatal("环境变量 SOLANA_RPC_URL 未设置")
//	}
//
//	log.Printf("成功加载环境变量: RPC_URL=%s", RPC_URL)
//}

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

func ExecuteTrade(action common.TradeAction, mint string, amount float64, sellPercent string, denominatedInSol bool, slippage int, priorityFee float64, pool common.PoolType) (string, error) {
	// 解析私钥（只解析一次）
	privateKey, err := solana.PrivateKeyFromBase58(PRIVATE_KEY)
	if err != nil {
		return "", fmt.Errorf("解析私钥失败: %v", err)
	}
	publicKey := privateKey.PublicKey()
	var request *TradeRequest
	if sellPercent != "" && denominatedInSol == false {
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
	data.Set("publicKey", request.PublicKey)
	data.Set("action", string(request.Action))
	data.Set("mint", request.Mint)
	data.Set("amount", fmt.Sprintf("%f", request.Amount))
	data.Set("denominatedInSol", request.DenominatedInSol)
	data.Set("slippage", fmt.Sprintf("%d", request.Slippage))
	data.Set("priorityFee", fmt.Sprintf("%f", request.PriorityFee))
	data.Set("pool", string(request.Pool))

	// 添加请求详情日志
	log.Printf("交易请求详情:")
	log.Printf("- 公钥: %s", request.PublicKey)
	log.Printf("- 动作: %s", request.Action)
	log.Printf("- 代币地址: %s", request.Mint)
	log.Printf("- 数量: %f", request.Amount)
	log.Printf("- 是否以SOL计价: %s", request.DenominatedInSol)
	log.Printf("- 滑点: %d", request.Slippage)
	log.Printf("- 优先费用: %f", request.PriorityFee)
	log.Printf("- 池类型: %s", request.Pool)

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
	
	// 检查并修复程序ID
	for i, inst := range tx.Message.Instructions {
		// 如果是第2个指令，并且程序ID是AToken程序，则修改为Token程序
		if i == 2 && tx.Message.AccountKeys[inst.ProgramIDIndex].String() == "ATokenGPvbdGVxr1b2hvZbsiqW5xWH25efTNsLJA8knL" {
			// 找到Token程序的索引
			for j, acc := range tx.Message.AccountKeys {
				if acc.String() == "TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA" {
					inst.ProgramIDIndex = uint16(j)
					log.Printf("修复指令 %d 的程序ID索引为 %d", i, j)
					break
				}
			}
		}
	}

	for i, inst := range tx.Message.Instructions {
		log.Printf("指令 %d 详情:", i)
		log.Printf("- 程序ID索引: %d (程序ID: %s)", inst.ProgramIDIndex, tx.Message.AccountKeys[inst.ProgramIDIndex].String())
		log.Printf("- 账户数量: %d", len(inst.Accounts))
		log.Printf("- 数据长度: %d", len(inst.Data))
		log.Printf("- 账户列表:")
		for j, accIndex := range inst.Accounts {
			log.Printf("  - 账户索引 %d: %d (账户: %s)", j, accIndex, tx.Message.AccountKeys[accIndex].String())
		}
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
