package main

import (
	"flag"
	"net/http"
	_ "net/http/pprof"

	"go.uber.org/zap"
)

var configFile = flag.String("config", "", "Specifies the config file to use")

func main() {

	logger, err := zap.NewDevelopment()
	if err != nil {
		panic("couldn't initialize logging")
	}
	sugar := logger.Sugar()
	go func() {
		sugar.Infow("starting the pprof server", "err", http.ListenAndServe("localhost:6060", nil))
	}()

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
