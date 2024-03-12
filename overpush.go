package main

import (
	"github.com/mrusme/overpush/api"
	"github.com/mrusme/overpush/lib"

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
		logger, _ = zap.NewDevelopment()
	} else {
		logger, _ = zap.NewProduction()
	}
	defer logger.Sync()
	// TODO: Use sugarLogger
	// sugar := logger.Sugar()

	apiServer, err := api.New(&config, logger)
	if err != nil {
		panic(err)
	}

	apiServer.Run()
}
