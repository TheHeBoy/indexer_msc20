package main

import (
	"indexer/config"
	"indexer/handlers"
	"indexer/log"
	"indexer/model"
	"indexer/mq"
	"os"
	"os/signal"
)

func main() {

	oss := make(chan os.Signal)

	// 初始化配置
	config.InitConfig()
	// 初始化日志
	log.InitLogger()
	// 初始化数据库
	model.InitDB()
	// 初始化消息队列
	mq.InitMQ()
	handlers.InitHandler()

	var logger = handlers.GetLogger()
	logger.Info("start indexer")

	// 从数据中拉取数据
	if handlers.DataSourceType == "rpc" {
		go func() {
			handlers.StartFetch()
		}()
	}

	stop := func() {
		logger.Info("app is stopping")
		handlers.StopFetch()
		mq.Close()
		logger.Info("app stopped.")
	}

	// 监听信号
	signal.Notify(oss, os.Interrupt, os.Kill)

	for {
		select {
		case <-oss: // kill -9 pid，no effect
			logger.Info("stopped by system...")
			stop()
			logger.Info("gracefully stopped.")
			return
		case <-handlers.QuitChan:
			logger.Info("stopped by app.")
			stop()
			logger.Info("app is auto stopped.")
			return
		}
	}
}
