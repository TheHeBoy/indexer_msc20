package model

import (
	"database/sql"
	"fmt"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"indexer/config"
	"time"
)

// DB 对象
var DB *gorm.DB
var SQLDB *sql.DB

func InitDB() {
	synCfg := config.Cfg.Section("db")

	var dbConfig gorm.Dialector
	// 构建 DSN 信息
	dsn := fmt.Sprintf("%v:%v@tcp(%v:%v)/%v?charset=%v&parseTime=True&multiStatements=true&loc=Local",
		synCfg.Key("username"),
		synCfg.Key("password"),
		synCfg.Key("host"),
		synCfg.Key("port"),
		synCfg.Key("database"),
		"utf8mb4",
	)
	dbConfig = mysql.New(mysql.Config{
		DSN: dsn,
	})

	// 使用 gorm.Open 连接数据库
	var err error
	DB, err = gorm.Open(dbConfig, &gorm.Config{
		Logger: logger.Default.LogMode(logger.Error),
	})
	// 处理错误
	if err != nil {
		fmt.Println(err.Error())
	}

	// 获取底层的 sqlDB
	SQLDB, err = DB.DB()
	if err != nil {
		fmt.Println(err.Error())
	}

	// 设置最大连接数
	SQLDB.SetMaxOpenConns(100)
	// 设置最大空闲连接数
	SQLDB.SetMaxIdleConns(25)
	// 设置每个链接的过期时间
	SQLDB.SetConnMaxLifetime(time.Duration(300) * time.Second)

	var tables []interface{}
	tables = append(tables, &EvmLog{})
	tables = append(tables, &Holder{})
	tables = append(tables, &List{})
	tables = append(tables, &Msc20{})
	tables = append(tables, &Token{})
	tables = append(tables, &Inscription{})
	tables = append(tables, &Transaction{})

	if synCfg.Key("refresh").MustBool(false) {
		err := DB.Migrator().DropTable(tables...)
		if err != nil {
			panic(err.Error())
		}
	}

	err = DB.AutoMigrate(tables...)
	if err != nil {
		panic(err.Error())
	}
}
