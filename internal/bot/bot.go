package bot

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"pump_auto/internal/execctor"
	"pump_auto/internal/model"
	"pump_auto/internal/queue"
	"sync"
	"time"

	"pump_auto/internal/analyzer"

	"github.com/gorilla/websocket"
)

type Bot struct {
	stopChan      chan struct{} // Channel to signal listener to stop
	mutex         sync.Mutex    // 互斥锁，用于保护共享状态
	isBuying      bool          // State to prevent multiple buys
	isBought      bool          // State to track if a token has been bought in a cycle
	ctx           context.Context
	cancelFunc    context.CancelFunc
	workerPool    chan struct{}           // 工作池通道，用于限制并发工作线程数
	workerWg      sync.WaitGroup          // 等待组，用于等待所有工作线程完成
	tradeExecutor *execctor.TradeExecutor // 交易执行器
}

// 创建新的Bot实例
func NewBot() *Bot {
	ctx, cancel := context.WithCancel(context.Background())
	return &Bot{
		stopChan:      make(chan struct{}),
		isBuying:      false,
		isBought:      false,
		ctx:           ctx,
		cancelFunc:    cancel,
		workerPool:    make(chan struct{}, 26),     // 创建容量为26的工作池
		tradeExecutor: execctor.NewTradeExecutor(), // 创建交易执行器
	}
}

// 修改监听实现代码
func (b *Bot) RunListener() error {
	log.Println("开始监听pump.fun上的新池...")

	// 添加重连逻辑
	maxRetries := 5
	retryCount := 0
	retryDelay := time.Second * 3

	for {
		select {
		case <-b.ctx.Done():
			// 等待所有工作线程完成
			b.workerWg.Wait()
			return nil
		default:
			// 建立连接
			log.Println("正在连接到WebSocket服务...")
			ws, _, err := websocket.DefaultDialer.Dial("wss://pumpportal.fun/api/data", nil)
			if err != nil {
				retryCount++
				if retryCount > maxRetries {
					return fmt.Errorf("连接WebSocket服务失败，已达到最大重试次数: %w", err)
				}
				log.Printf("连接WebSocket服务失败: %v, 将在 %v 后重试 (%d/%d)", err, retryDelay, retryCount, maxRetries)
				time.Sleep(retryDelay)
				continue
			}

			// 重置重试计数
			retryCount = 0
			log.Println("成功连接到pumpportal.fun WebSocket API")

			// 设置关闭处理函数
			defer func() {
				if err := ws.Close(); err != nil {
					log.Printf("关闭WebSocket连接失败: %v", err)
				} else {
					log.Println("WebSocket连接已正常关闭")
				}
			}()

			// 2. 订阅新代币创建事件
			if err := subscribeToNewTokens(ws); err != nil {
				log.Printf("订阅新代币事件失败: %v, 将重新连接", err)
				// 关闭当前连接并重新开始
				ws.Close()
				time.Sleep(retryDelay)
				continue
			}

			// 3. 订阅迁移事件
			if err := subscribeToMigrations(ws); err != nil {
				log.Printf("订阅迁移事件失败: %v, 将重新连接", err)
				// 关闭当前连接并重新开始
				ws.Close()
				time.Sleep(retryDelay)
				continue
			}

			// 连续超时计数
			consecutiveTimeouts := 0
			maxConsecutiveTimeouts := 3

			// 消息处理循环
		messageLoop:
			for {
				select {
				case <-b.ctx.Done():
					// 等待所有工作线程完成
					b.workerWg.Wait()
					return nil
				default:
					// 读取消息
					_, message, err := ws.ReadMessage()
					if err != nil {
						if websocket.IsUnexpectedCloseError(err) {
							log.Printf("WebSocket连接已关闭: %v, 将重新连接", err)
							break messageLoop
						}

						// 检查是否是超时错误
						if err.Error() == "i/o timeout" || err.Error() == "read tcp: i/o timeout" ||
							err.Error() == "read tcp 10.10.15.196:60029->35.194.64.63:443: i/o timeout" {
							consecutiveTimeouts++
							if consecutiveTimeouts >= maxConsecutiveTimeouts {
								log.Printf("连续超时次数过多: %d/%d, 将重新连接", consecutiveTimeouts, maxConsecutiveTimeouts)
								break messageLoop
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
						// 如果消息没有method字段，可能是代币事件
						formattedMsg := model.FormatTokenEvent(message)
						log.Println(formattedMsg)

						// 同时解析为TokenEvent结构体，以便正确获取字段
						var tokenEvent model.TokenEvent
						if err := json.Unmarshal(message, &tokenEvent); err != nil {
							log.Printf("解析为TokenEvent失败: %v", err)
							continue
						}

						// 检查是否是新代币创建事件(txType=create)
						if tokenEvent.TxType == "create" {
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

			// 连接断开或需要重连，等待一段时间再尝试
			log.Println("准备重新连接WebSocket服务...")
			time.Sleep(retryDelay)
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

// 买入代币
func (b *Bot) buyToken(tokenAddress string, tokenSymbol string, tokenName string, tokenURI string) {
	b.mutex.Lock()
	if b.isBuying || b.isBought {
		b.mutex.Unlock()
		log.Printf("已在买入状态或已买过代币，跳过买入 %s", tokenAddress)
		return
	}

	b.isBuying = true
	b.mutex.Unlock()

	// TODO: 计算实际要买入的金额
	buyAmount := 0.1 // 示例金额: 0.1 SOL
	price := 1.0     // 示例价格

	// 使用交易执行器执行买入操作
	err := b.tradeExecutor.BuyToken(tokenAddress, tokenSymbol, buyAmount, price, 6)

	b.mutex.Lock()
	b.isBuying = false
	if err == nil {
		b.isBought = true
		log.Printf("成功买入代币 %s (%s)", tokenAddress, tokenName)
	} else {
		log.Printf("买入代币 %s (%s) 失败: %v", tokenAddress, tokenName, err)
	}
	b.mutex.Unlock()
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

			// 使用代币名称和符号（优先使用元数据中的信息）
			tokenSymbolToUse := metadata.Symbol
			if tokenSymbolToUse == "" {
				tokenSymbolToUse = tokenSymbol
			}

			tokenNameToUse := metadata.Name
			if tokenNameToUse == "" {
				tokenNameToUse = tokenName
			}

			// 创建购买消息并发送到购买队列
			buyMsg := model.NewBuyMessage(
				tokenAddress,
				tokenSymbolToUse,
				tokenNameToUse,
				tokenURI,
			)

			// 添加元数据信息到额外数据
			buyMsg.ExtraData["description"] = metadata.Description
			if metadata.Twitter != "" {
				buyMsg.ExtraData["twitter"] = metadata.Twitter
			}
			if metadata.Website != "" {
				buyMsg.ExtraData["website"] = metadata.Website
			}

			// 发送到购买队列
			queue.GetBuyQueue().SendMessage(buyMsg)
		}
	} else {
		log.Printf("代币 %s 缺少元数据URI，无法进行筛选", tokenAddress)
	}

	log.Printf("工作线程完成处理代币: %s", tokenAddress)
}

// 订阅新代币创建事件
func subscribeToNewTokens(ws *websocket.Conn) error {
	payload := map[string]interface{}{
		"method": "subscribeNewToken",
	}

	return sendSubscription(ws, payload)
}

// 订阅迁移事件
func subscribeToMigrations(ws *websocket.Conn) error {
	payload := map[string]interface{}{
		"method": "subscribeMigration",
	}

	return sendSubscription(ws, payload)
}

// 订阅账户交易事件
func subscribeToAccountTrades(ws *websocket.Conn, accounts []string) error {
	payload := map[string]interface{}{
		"method": "subscribeAccountTrade",
		"keys":   accounts,
	}

	return sendSubscription(ws, payload)
}

// 订阅代币交易事件
func subscribeToTokenTrades(ws *websocket.Conn, tokens []string) error {
	payload := map[string]interface{}{
		"method": "subscribeTokenTrade",
		"keys":   tokens,
	}

	return sendSubscription(ws, payload)
}

// 发送订阅请求
func sendSubscription(ws *websocket.Conn, payload map[string]interface{}) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("序列化订阅请求失败: %w", err)
	}

	if err := ws.WriteMessage(websocket.TextMessage, data); err != nil {
		return fmt.Errorf("发送订阅请求失败: %w", err)
	}

	log.Printf("已发送订阅请求: %s", string(data))
	return nil
}

// 关闭Bot并清理资源
func (b *Bot) Close() {
	b.cancelFunc()
	close(b.stopChan)

	// 停止交易执行器
	b.tradeExecutor.Stop()

	// 等待所有工作线程完成
	b.workerWg.Wait()
	log.Println("所有工作线程已完成，Bot已关闭")
}

// ResetBuyState 重置购买状态，开始新的购买周期
func (b *Bot) ResetBuyState() {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	b.isBought = false
	log.Println("已重置购买状态，可以开始新的购买周期")
}
