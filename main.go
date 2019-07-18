package main

import (
	"os"

	"go.uber.org/zap"
)

func main() {
	if len(os.Args) < 2 {
		panic("failed to provide a config file")
	}
	configFile := os.Args[1]

	logger, err := zap.NewDevelopment()
	if err != nil {
		panic("couldn't initialize logging")
	}
	sugar := logger.Sugar()

	bb, err := NewBufferbloater(configFile, sugar)
	if err != nil {
		sugar.Fatalw("failed to create bufferbloater",
			"error", err)
	}

	sugar.Infof("ok %+v", &bb)
}
