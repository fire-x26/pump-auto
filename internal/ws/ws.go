package ws

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

var (
	globalWS     *websocket.Conn
	wsMutex      sync.Mutex
	stopChan     chan struct{}
	reconnectDelay = time.Second * 3
	maxRetries    = 5
)

// InitGlobalWS 初始化全局WebSocket连接
func InitGlobalWS() error {
	wsMutex.Lock()
	defer wsMutex.Unlock()

	if globalWS != nil {
		return nil
	}

	stopChan = make(chan struct{})
	return connectAndSubscribe()
}

// GetGlobalWS 获取全局WebSocket连接
func GetGlobalWS() *websocket.Conn {
	wsMutex.Lock()
	defer wsMutex.Unlock()
	return globalWS
}

// connectAndSubscribe 建立连接并订阅新代币事件
func connectAndSubscribe() error {
	ws, _, err := websocket.DefaultDialer.Dial("wss://pumpportal.fun/api/data", nil)
	if err != nil {
		return fmt.Errorf("连接WebSocket服务失败: %w", err)
	}

	globalWS = ws

	// 订阅新代币创建事件
	payload := map[string]interface{}{
		"method": "subscribeNewToken",
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("序列化订阅请求失败: %w", err)
	}

	if err := ws.WriteMessage(websocket.TextMessage, data); err != nil {
		return fmt.Errorf("发送订阅请求失败: %w", err)
	}

	log.Println("成功连接到pumpportal.fun WebSocket API并订阅新代币事件")
	return nil
}

// SubscribeToTokenTrades 订阅代币交易事件
func SubscribeToTokenTrades(tokens []string) error {
	ws := GetGlobalWS()
	if ws == nil {
		return fmt.Errorf("WebSocket连接未建立")
	}

	payload := map[string]interface{}{
		"method": "subscribeTokenTrade",
		"keys":   tokens,
	}

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

// UnsubscribeToTokenTrades 取消订阅代币交易事件
func UnsubscribeToTokenTrades(tokens []string) error {
	ws := GetGlobalWS()
	if ws == nil {
		return fmt.Errorf("WebSocket连接未建立")
	}

	payload := map[string]interface{}{
		"method": "unsubscribeTokenTrade",
		"keys":   tokens,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("序列化取消订阅请求失败: %w", err)
	}

	if err := ws.WriteMessage(websocket.TextMessage, data); err != nil {
		return fmt.Errorf("发送取消订阅请求失败: %w", err)
	}

	log.Printf("已发送取消订阅请求: %s", string(data))
	return nil
}

// Close 关闭全局WebSocket连接
func Close() {
	wsMutex.Lock()
	defer wsMutex.Unlock()

	if globalWS != nil {
		globalWS.Close()
		globalWS = nil
	}
	close(stopChan)
}
