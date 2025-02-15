package log

import (
	"github.com/sirupsen/logrus"
	"io"
	"os"
)

var Logger *logrus.Logger

func InitLogger() {
	writerStd := os.Stdout
	writerFile, err := os.OpenFile("logs.txt", os.O_WRONLY|os.O_CREATE, 0755)
	if err != nil {
		logrus.Fatalf("create file logs.txt failed: %v", err)
	}

	Logger = logrus.New()
	Logger.SetLevel(logrus.InfoLevel)
	Logger.SetFormatter(&logrus.TextFormatter{})
	Logger.SetOutput(io.MultiWriter(writerStd, writerFile))
}
