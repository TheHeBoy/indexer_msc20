package handlers

import (
	"github.com/sirupsen/logrus"
	"indexer/config"
	"indexer/log"
)

var logger *logrus.Logger
var DataSourceType string
var QuitChan = make(chan bool)

func InitHandler() {
	logger = log.Logger
	initDataSource()
	initFetch()
	initCacheData()
}

func initDataSource() {
	dsCfg := config.Cfg.Section("data-source")
	DataSourceType = dsCfg.Key("type").String()
	dsUri := dsCfg.Key("uri").String()

	if DataSourceType == "rpc" {
		fetchUrl = dsUri
	} else {
		panic("error data source type")
	}
}

// todo 增加 token、list 和 holder 的缓存
func initCacheData() {}

func GetLogger() *logrus.Logger {
	return logger
}
