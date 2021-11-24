package main

import (
	"flag"

	"go.uber.org/zap"
)

var configFile = flag.String("config", "", "Specifies the config file to use")

func main() {
	logger, err := zap.NewDevelopment()
	if err != nil {
		panic("couldn't initialize logging")
	}
	sugar := logger.Sugar()

	flag.Parse()
	if *configFile == "" {
		sugar.Fatalw("configuration file not specified")
	}

	bb, err := NewBufferbloater(*configFile, sugar)
	if err != nil {
		sugar.Fatalw("failed to create bufferbloater",
			"error", err)
	}

	bb.Run()

	sugar.Infof("ok %+v", &bb)
}
