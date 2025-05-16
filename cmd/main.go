package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"pump_auto/internal/bot"
	"pump_auto/internal/model"
	"pump_auto/internal/queue"
	// "pump_auto/internal/ui" // Placeholder for a more complex CLI menu if needed
)

func main() {
	// 定义命令行参数
	testMode := flag.Bool("test", false, "是否运行策略测试模式")
	flag.Parse()

	// 初始化消息队列
	queue.InitGlobalQueues()
	log.Println("消息队列系统已初始化")

	// 初始化bot
	sniperBot := bot.NewBot()

	if *testMode {
		// 运行交易策略测试
		runStrategyTest(sniperBot)
		return
	}

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

// 运行交易策略测试
func runStrategyTest(sniperBot *bot.Bot) {
	fmt.Println("=== 开始交易策略测试 ===")

	// 模拟代币地址和符号
	tokenAddress := "SimulatedToken12345"
	tokenSymbol := "SIM"
	tokenName := "Simulation Token"
	tokenURI := "https://example.com/token/metadata.json"

	// 直接发送测试购买消息到购买队列
	buyMsg := model.NewBuyMessage(tokenAddress, tokenSymbol, tokenName, tokenURI)
	queue.GetBuyQueue().SendMessage(buyMsg)

	log.Println("已发送测试购买消息，等待处理...")

	// 等待足够长的时间，让所有消息得到处理
	time.Sleep(30 * time.Second)

	fmt.Println("=== 交易策略测试完成 ===")

	// 关闭系统
	queue.GetBuyQueue().Stop()
	queue.GetSellQueue().Stop()
	sniperBot.Close()
}
