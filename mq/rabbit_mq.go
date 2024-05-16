package mq

import (
	"context"
	"encoding/json"
	amqp "github.com/rabbitmq/amqp091-go"
	"gopkg.in/ini.v1"
	"indexer/config"
	"indexer/log"
	"indexer/model"
	"strings"
	"time"
)

var mqCfg *ini.Section
var conn *amqp.Connection
var ch *amqp.Channel
var exchangeName = "logs_topic"
var routingKey = "list"
var contract string

func InitMQ() {
	mqCfg = config.Cfg.Section("mq")
	contract = mqCfg.Key("contract_address").String()

	if !IsEnable() {
		return
	}
	var err error
	conn, err = amqp.Dial(mqCfg.Key("uri").String())
	if err != nil {
		panic(err)
	}

	ch, err = conn.Channel()
	if err != nil {
		panic(err)
	}
	err = ch.ExchangeDeclare(
		exchangeName, // name
		"topic",      // type
		true,         // durable
		false,        // auto-deleted
		false,        // internal
		false,        // no-wait
		nil,          // arguments
	)
	if err != nil {
		panic(err)
	}

}

func SendListMessage(list model.List) {
	if !strings.EqualFold(list.Exchange, contract) {
		return
	}
	log.Logger.Infof("SendListMessage %+v", list)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	jsonData, err := json.Marshal(list)
	if err != nil {
		log.Logger.Errorf("Failed to marshal %+v", err)
		return
	}

	err = ch.PublishWithContext(ctx,
		exchangeName, // exchange
		routingKey,   // routing key
		false,        // mandatory
		false,        // immediate
		amqp.Publishing{
			ContentType: "application/json",
			Body:        jsonData,
		})
	if err != nil {
		log.Logger.Errorf("Failed to publish a message %+v", err)
	}
}

func Close() {
	conn.Close()
	ch.Close()
}

func IsEnable() bool {
	return mqCfg.Key("enable").MustBool()
}
