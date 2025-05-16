package queue

import (
	"log"
	"pump_auto/internal/model"
	"sync"
)

// 消息队列管理器
type MessageQueue struct {
	name     string                   // 队列名称
	messages chan *model.QueueMessage // 消息通道
	handlers []MessageHandler         // 消息处理器
	mutex    sync.RWMutex             // 读写锁
}

// 消息处理器接口
type MessageHandler interface {
	HandleMessage(msg *model.QueueMessage)
}

// 创建新消息队列
func NewMessageQueue(name string, bufferSize int) *MessageQueue {
	return &MessageQueue{
		name:     name,
		messages: make(chan *model.QueueMessage, bufferSize),
		handlers: make([]MessageHandler, 0),
	}
}

// 注册消息处理器
func (q *MessageQueue) RegisterHandler(handler MessageHandler) {
	q.mutex.Lock()
	defer q.mutex.Unlock()
	q.handlers = append(q.handlers, handler)
}

// 发送消息到队列
func (q *MessageQueue) SendMessage(msg *model.QueueMessage) {
	select {
	case q.messages <- msg:
		log.Printf("消息已发送到队列 %s: TokenAddress=%s", q.name, msg.TokenAddress)
	default:
		log.Printf("警告: 队列 %s 已满，消息被丢弃: TokenAddress=%s", q.name, msg.TokenAddress)
	}
}

// 启动消息处理
func (q *MessageQueue) Start() {
	go func() {
		for msg := range q.messages {
			q.mutex.RLock()
			handlers := q.handlers
			q.mutex.RUnlock()

			for _, handler := range handlers {
				go handler.HandleMessage(msg)
			}
		}
	}()

	log.Printf("队列 %s 已启动", q.name)
}

// 停止消息处理
func (q *MessageQueue) Stop() {
	close(q.messages)
	log.Printf("队列 %s 已停止", q.name)
}

// 全局队列管理器
var (
	BuyQueue  *MessageQueue
	SellQueue *MessageQueue
	once      sync.Once
)

// 初始化全局队列
func InitGlobalQueues() {
	once.Do(func() {
		BuyQueue = NewMessageQueue("buy_queue", 100)
		SellQueue = NewMessageQueue("sell_queue", 100)

		BuyQueue.Start()
		SellQueue.Start()

		log.Println("全局消息队列已初始化")
	})
}

// 获取购买队列
func GetBuyQueue() *MessageQueue {
	if BuyQueue == nil {
		InitGlobalQueues()
	}
	return BuyQueue
}

// 获取卖出队列
func GetSellQueue() *MessageQueue {
	if SellQueue == nil {
		InitGlobalQueues()
	}
	return SellQueue
}
