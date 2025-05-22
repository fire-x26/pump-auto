package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"pump_auto/internal/bot"
	"pump_auto/internal/queue"
	"syscall"
	// "pump_auto/internal/ui" // Placeholder for a more complex CLI menu if needed
)

func main() {
	// 定义命令行参数

	// 初始化消息队列
	queue.InitGlobalQueues()
	log.Println("消息队列系统已初始化")

	// 初始化bot
	sniperBot := bot.NewBot()

	// 启动主要bot逻辑（例如，监听器）
	go func() {
		if err := sniperBot.RunListener(); err != nil {
			log.Printf("Bot监听器错误: %v", err)
		}
	}()

	fmt.Println("Bot系统已启动，所有模块正在运行. 按CTRL+C退出.")

	// 等待终止信号
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c

	fmt.Println("正在关闭Bot系统...")

	// 停止所有组件
	queue.GetBuyQueue().Stop()
	queue.GetSellQueue().Stop()
	sniperBot.Close()

	fmt.Println("Bot系统已正常关闭.")
}
