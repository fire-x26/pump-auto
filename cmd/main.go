package main

import (
	"os"
	"os/signal"
	"pump_auto/internal/bot"
	"pump_auto/internal/common"
	"pump_auto/internal/queue"
	"syscall"
	// "pump_auto/internal/ui" // Placeholder for a more complex CLI menu if needed
)

func main() {
	// 设置日志级别
	// 可以通过环境变量控制日志级别
	logLevel := os.Getenv("LOG_LEVEL")
	if logLevel == "" {
		logLevel = "debug" // 默认使用 debug 级别
	}
	common.SetLogLevel(logLevel)
	common.Log.Info("日志系统初始化完成")

	// 初始化消息队列
	queue.InitGlobalQueues()
	common.Log.Info("消息队列系统已初始化")

	// 初始化bot
	sniperBot := bot.NewBot()

	// 启动主要bot逻辑（例如，监听器）
	go func() {
		if err := sniperBot.RunListener(); err != nil {
			common.Log.WithError(err).Error("Bot监听器错误")
		}
	}()

	common.Log.Info("Bot系统已启动，所有模块正在运行. 按CTRL+C退出.")

	// 等待终止信号
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c

	common.Log.Info("正在关闭Bot系统...")

	// 停止所有组件
	queue.GetBuyQueue().Stop()
	queue.GetSellQueue().Stop()
	sniperBot.Close()

	common.Log.Info("Bot系统已正常关闭.")
}
