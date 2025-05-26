package bot

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"pump_auto/internal/chainTx"
	"pump_auto/internal/common"
	"pump_auto/internal/execctor"
	"pump_auto/internal/model"
	"pump_auto/internal/ws"
	"sync"
	"time"

	"pump_auto/internal/analyzer"

	"github.com/gagliardetto/solana-go"
	"github.com/gorilla/websocket"
)

// PumpfunBuyInstruction 表示 Pumpfun 程序的购买指令
type PumpfunBuyInstruction struct {
	Token                  solana.PublicKey
	Signer                 solana.PublicKey
	BondingCurve           solana.PublicKey
	AssociatedBondingCurve solana.PublicKey
	UserWallet             solana.PublicKey
	Input                  *BuyInstruction
}

// BuyInstruction 表示购买指令的具体参数
type BuyInstruction struct {
	MaxAmountIn uint64
	AmountOut   uint64
}

type Bot struct {
	stopChan      chan struct{} // Channel to signal listener to stop
	mutex         sync.Mutex    // 互斥锁，用于保护共享状态
	ctx           context.Context
	cancelFunc    context.CancelFunc
	workerPool    chan struct{}           // 工作池通道，用于限制并发工作线程数
	workerWg      sync.WaitGroup          // 等待组，用于等待所有工作线程完成
	tradeExecutor *execctor.TradeExecutor // 交易执行器
	heldTokens    map[string]chan bool    // 修改为map[string]chan bool，用于存储代币的心跳通道
}

// 创建新的Bot实例
func NewBot() *Bot {
	ctx, cancel := context.WithCancel(context.Background())
	b := &Bot{
		stopChan:   make(chan struct{}),
		ctx:        ctx,
		cancelFunc: cancel,
		workerPool: make(chan struct{}, 1),     // 修改此处，创建容量为2的工作池
		heldTokens: make(map[string]chan bool), // 初始化持有的代币
	}
	b.tradeExecutor = execctor.NewTradeExecutor(b.RemoveHeldToken) // 创建交易执行器并传入回调
	return b
}

// SubscribeToTokenTrade 订阅代币交易
func (b *Bot) SubscribeToTokenTrade(tokenAddress string) error {
	return ws.SubscribeToTokenTrades([]string{tokenAddress})
}

// 修改监听实现代码
func (b *Bot) RunListener() error {
	log.Println("开始监听pump.fun上的新池...")

	// 初始化全局WebSocket连接
	if err := ws.InitGlobalWS(); err != nil {
		return fmt.Errorf("初始化WebSocket连接失败: %w", err)
	}

	// 连续超时计数
	consecutiveTimeouts := 0
	maxConsecutiveTimeouts := 3

	// 消息处理循环
	for {
		select {
		case <-b.ctx.Done():
			// 等待所有工作线程完成
			b.workerWg.Wait()
			return nil
		default:
			wsConn := ws.GetGlobalWS()
			if wsConn == nil {
				log.Println("WebSocket连接未建立，尝试重新连接...")
				if err := ws.InitGlobalWS(); err != nil {
					log.Printf("重新连接WebSocket失败: %v", err)
					time.Sleep(time.Second * 3)
					continue
				}
				continue
			}

			// 读取消息
			_, message, err := wsConn.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err) {
					log.Printf("WebSocket连接已关闭: %v, 将重新连接", err)
					continue
				}

				// 检查是否是超时错误
				if err.Error() == "i/o timeout" || err.Error() == "read tcp: i/o timeout" {
					consecutiveTimeouts++
					if consecutiveTimeouts >= maxConsecutiveTimeouts {
						log.Printf("连续超时次数过多: %d/%d, 将重新连接", consecutiveTimeouts, maxConsecutiveTimeouts)
						continue
					}
					log.Printf("读取消息超时 (%d/%d), 继续尝试...", consecutiveTimeouts, maxConsecutiveTimeouts)
					continue
				}

				// 其他错误
				log.Printf("读取消息出错: %v", err)
				continue
			}

			// 成功读取，重置超时计数
			consecutiveTimeouts = 0

			// 处理收到的消息
			var data map[string]interface{}
			if err := json.Unmarshal(message, &data); err != nil {
				log.Printf("解析消息失败: %v", err)
				continue
			}

			// 以更易读的方式打印消息
			if _, ok := data["method"]; !ok {
				// 解析为TokenEvent结构体，用于判断消息类型
				var tokenEvent model.TokenEvent
				if err := json.Unmarshal(message, &tokenEvent); err != nil {
					log.Printf("解析为TokenEvent失败: %v", err)
					continue
				}

				// 如果是交易记录消息(buy或sell)，则转发给交易执行器更新价格
				if tokenEvent.TxType == "buy" || tokenEvent.TxType == "sell" {
					go func(msg []byte) {
						b.tradeExecutor.ProcessTradeMessage(msg)
					}(message)

					// 检查是否是持有的代币的交易消息
					if ch := b.getTokenChannel(tokenEvent.Mint); ch != nil {
						select {
						case ch <- true:
							// 成功发送心跳
						case <-b.ctx.Done():
							// 上下文已取消，忽略
						}
					}
				}

				// 检查是否是新代币创建事件(txType=create)
				if tokenEvent.TxType == "create" {

					// 检查当前持有的代币数量
					b.mutex.Lock()
					heldTokensCount := len(b.heldTokens)
					b.mutex.Unlock()

					if heldTokensCount >= common.MAX_HOLD_TOKEN {
						log.Printf("当前已持有 %d 个代币，暂停处理新代币创建事件", heldTokensCount)
						continue
					}
					formattedMsg := model.FormatTokenEvent(message)
					log.Println(formattedMsg)
					// 将tokenEvent转换为map，以便与现有代码兼容
					tokenData := map[string]interface{}{
						"params": map[string]interface{}{
							"address": tokenEvent.Mint,
							"uri":     tokenEvent.Uri,
						},
						"name":   tokenEvent.Name,
						"symbol": tokenEvent.Symbol,
					}

					// 在协程中处理，避免阻塞主消息循环
					go b.processNewToken(tokenData)
				}
			} else {
				// 处理系统消息或订阅确认消息
				if method, ok := data["method"].(string); ok {
					log.Printf("收到系统消息: %s", method)
				}
			}
		}
	}
}

func fetchMetadata(uri string) (*model.TokenMetadata, error) {
	// 发起HTTP请求获取元数据
	resp, err := http.Get(uri)
	if err != nil {
		log.Printf("HTTP请求失败: %v", err)
		return nil, err
	}
	defer resp.Body.Close()

	// 读取响应数据
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("读取响应内容失败: %v", err)
		return nil, err
	}

	// 解析JSON数据
	var metadata model.TokenMetadata
	if err := json.Unmarshal(body, &metadata); err != nil {
		log.Printf("解析JSON失败: %v", err)
		return nil, err
	}

	return &metadata, nil
}

// 处理新代币的工作线程
func (b *Bot) processNewToken(data map[string]interface{}) {
	// 在工作线程启动前获取工作池槽位
	b.workerPool <- struct{}{}
	b.workerWg.Add(1)

	defer func() {
		// 完成时释放工作线程槽位
		<-b.workerPool
		b.workerWg.Done()
	}()

	// 从data中提取代币信息
	var tokenAddress, tokenURI, tokenName, tokenSymbol string

	if params, ok := data["params"].(map[string]interface{}); ok {
		if addr, ok := params["address"].(string); ok {
			tokenAddress = addr
		}
		if uri, ok := params["uri"].(string); ok {
			tokenURI = uri
		}
	}

	// 尝试获取代币名称和符号
	if name, ok := data["name"].(string); ok {
		tokenName = name
	}
	if symbol, ok := data["symbol"].(string); ok {
		tokenSymbol = symbol
	}

	if tokenAddress == "" {
		log.Println("无法获取代币地址，跳过处理")
		return
	}

	log.Printf("工作线程开始处理代币: %s (%s), URI: %s", tokenAddress, tokenName, tokenURI)

	// 使用过滤器检查代币是否满足条件
	if tokenURI != "" {
		// 获取代币元数据
		metadata, err := fetchMetadata(tokenURI)
		if err != nil {
			log.Printf("获取代币 %s 的元数据失败: %v", tokenAddress, err)
			return
		}

		// 确保元数据中包含必要的信息
		if metadata.Name == "" && tokenName != "" {
			metadata.Name = tokenName
		}
		if metadata.Symbol == "" && tokenSymbol != "" {
			metadata.Symbol = tokenSymbol
		}

		log.Printf("成功获取代币 %s (%s) 的元数据: Name=%s, Symbol=%s, Description=%s",
			tokenAddress, tokenName, metadata.Name, metadata.Symbol, metadata.Description)

		// 获取默认过滤器配置
		config := analyzer.DefaultConfig()

		// 处理代币并进行过滤
		result := analyzer.ProcessToken(tokenAddress, tokenURI, metadata, config)

		// 打印筛选结果
		if result.IsFiltered {
			log.Printf("代币 %s (%s) 被过滤器拦截，原因: %v", tokenAddress, metadata.Name, result.FilteredBy)
			return
		} else {
			log.Printf("代币 %s (%s) 满足筛选条件，准备购买", tokenAddress, metadata.Name)
			req := &common.TradeReq{
				Action:           "buy",
				Mint:             tokenAddress,
				Amount:           0.001, // This is SOL amount
				DenominatedInSol: true,
				Slippage:         10,
				PriorityFee:      0.0005,
				Pool:             common.PUMP,
			}

			// metadata.Symbol 应该存在于 model.TokenMetadata 中
			_, err := b.buyToken(req.Mint, req.Amount, req.DenominatedInSol, req.Slippage, req.PriorityFee, req.Pool)
			if err != nil {
				log.Printf("购买代币 %s 失败,error: %v", tokenAddress, err)
			}

		}
	} else {
		log.Printf("代币 %s 缺少元数据URI，无法进行筛选", tokenAddress)
	}

	log.Printf("工作线程完成处理代币: %s", tokenAddress)
}

// 添加新的辅助方法
func (b *Bot) getTokenChannel(mint string) chan bool {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	return b.heldTokens[mint]
}

func (b *Bot) waitForTradeAndSellIfTimeout(mint string) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-b.getTokenChannel(mint):
			// 收到交易消息，重置计时器
			ticker.Reset(30 * time.Second)
		case <-ticker.C:
			// 每30秒检查一次
			log.Printf("代币 %s 在30秒内没有收到交易消息，执行卖出", mint)
			_, err := chainTx.SellToken(mint, 1, "100%", false, 20, 0.0005, common.PUMP)
			if err != nil {
				log.Printf("卖出代币 %s 失败: %v", mint, err)
			}
			b.RemoveHeldToken(mint)
			return
		case <-b.ctx.Done():
			b.RemoveHeldToken(mint)
			return
		}
	}
}

// 修改buyToken方法
func (b *Bot) buyToken(mint string, amount float64, denominatedInSol bool, slippage int, priorityFee float64, pool common.PoolType) (string, error) {
	b.mutex.Lock()
	if len(b.heldTokens) >= common.MAX_HOLD_TOKEN {
		b.mutex.Unlock()
		log.Printf("已持有最大数量的代币 ()，无法购买新的代币 %s", mint)
		return "", fmt.Errorf("已持有最大数量的代币 (2)，无法购买新的代币 %s", mint)
	}
	b.mutex.Unlock()
	time.Sleep(10 * time.Second)

	var sign string
	var err error

	// 开始重试循环
	sign, err = chainTx.BuyToken(mint, amount, denominatedInSol, slippage, priorityFee, pool)

	if err != nil {
		log.Printf("购买代币 %s 失败，已达到最大重试次数: %v", mint, err)
		return "", err
	}

	wsConn := ws.GetGlobalWS()
	if wsConn == nil {
		return "", fmt.Errorf("WebSocket连接未建立，无法购买代币 %s", mint)
	}

	// 创建心跳通道并存储
	b.mutex.Lock()
	b.heldTokens[mint] = make(chan bool, 1)
	b.mutex.Unlock()

	txSig := solana.MustSignatureFromBase58(sign)
	outAmount, err := chainTx.ParseTxSign(txSig)
	if err != nil {
		log.Printf("获取代币 %s 余额失败: %v", mint, err)
		return "", fmt.Errorf("获取代币余额失败: %v", err)
	}
	TokenBalance, err := chainTx.GetTokenBalance(mint)
	if err != nil || outAmount != TokenBalance {
		log.Printf("获取代币 %s 余额失败: %v,执行卖出", mint, err)
		_, _ = chainTx.SellToken(mint, 1, "100%", false, 20, 0.0005, common.PUMP)
		return "", fmt.Errorf("获取代币余额失败: %v,中断该代币的执行", err)
	}
	log.Printf("购买后代币 %s 余额: %f", mint, outAmount)

	err = ws.SubscribeToTokenTrades([]string{mint})
	if err == nil {
		b.mutex.Lock()
		b.mutex.Unlock()
		log.Printf("成功购买代币 %s 并添加到持有列表", mint)

		// 'amount' is the SOL amount intended to be spent
		b.tradeExecutor.ExpectBuyForToken(mint, amount, outAmount)

		// 启动超时检查
		go b.waitForTradeAndSellIfTimeout(mint)
	}
	return sign, err
}

// RemoveHeldToken 从持有代币列表中移除代币
func (b *Bot) RemoveHeldToken(tokenAddress string) {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	if ch, exists := b.heldTokens[tokenAddress]; exists {
		close(ch) // 关闭通道
		delete(b.heldTokens, tokenAddress)
		log.Printf("代币 %s 已从持有列表移除", tokenAddress)

		// 取消WebSocket订阅
		if err := ws.UnsubscribeToTokenTrades([]string{tokenAddress}); err != nil {
			log.Printf("取消订阅代币 %s 失败: %v", tokenAddress, err)
		} else {
			log.Printf("成功取消订阅代币 %s", tokenAddress)
		}
	}
}

// 关闭Bot并清理资源
func (b *Bot) Close() {
	b.cancelFunc()
	close(b.stopChan)

	// 停止交易执行器
	b.tradeExecutor.Stop()

	// 关闭全局WebSocket连接
	ws.Close()

	// 等待所有工作线程完成
	b.workerWg.Wait()
	log.Println("所有工作线程已完成，Bot已关闭")
}
