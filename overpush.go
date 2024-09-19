package main

import (
	"os"
	"os/signal"

	"github.com/mrusme/overpush/api"
	"github.com/mrusme/overpush/lib"
	"github.com/mrusme/overpush/worker"

	"go.uber.org/zap"
)

func init() {

}

func main() {
	var logger *zap.Logger

	config, err := lib.Cfg()
	if err != nil {
		panic(err)
	}

	if config.Debug == "true" {
		logcfg := zap.NewDevelopmentConfig()
		logcfg.OutputPaths = []string{"stdout"}
		logcfg.Level.SetLevel(zap.DebugLevel)
		logger, _ = logcfg.Build()
	} else {
		logcfg := zap.NewProductionConfig()
		logcfg.OutputPaths = []string{"stdout"}
		logcfg.Level.SetLevel(zap.InfoLevel)
		logger, _ = logcfg.Build()
	}
	defer logger.Sync()
	// TODO: Use sugarLogger
	// sugar := logger.Sugar()

	wrk, err := worker.New(&config, logger)
	go wrk.Run()

	apiServer, err := api.New(&config, logger)
	if err != nil {
		panic(err)
	}
	go apiServer.Run()

	quit := make(chan os.Signal)
	signal.Notify(quit, os.Interrupt)
	<-quit

	wrk.Shutdown()
	apiServer.Shutdown()
}
