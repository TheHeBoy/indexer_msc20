package config

import (
	"gopkg.in/ini.v1"
)

var Cfg *ini.File

func InitConfig() {
	var err error
	Cfg, err = ini.ShadowLoad("config.ini")
	if err != nil {
		panic("read config.ini file error: " + err.Error())
	}
}
