package main

import (
	"fmt"
	"os"
	"os/signal"

	"github.com/mrusme/overpush/api"
	"github.com/mrusme/overpush/config"
	"github.com/mrusme/overpush/worker"

	"go.uber.org/zap"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func init() {
}

func main() {
	var logger *zap.Logger
	var err error

	if len(os.Args) > 1 {
		fmt.Printf("Overpush %s (%s) %s\n", version, commit, date)
		os.Exit(1)
	}

	config, err := config.Cfg()
	if err != nil {
		panic(err)
	}

	if config.Debug == true {
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

	logger.Info("Overpush",
		zap.String("version", version),
		zap.String("commit", commit),
		zap.String("date", date),
	)

	var wrk *worker.Worker
	if wrk, err = worker.New(&config, logger); err != nil {
		panic(err)
	}
	go wrk.Run()

	var apiServer *api.API
	if apiServer, err = api.New(&config, logger); err != nil {
		wrk.Shutdown()
		panic(err)
	}
	if err = apiServer.LoadMiddlewares(); err != nil {
		wrk.Shutdown()
		panic(err)
	}
	apiServer.AttachRoutes()
	go apiServer.Run()

	if config.Server.Enable == false &&
		(config.Worker.Enable == false || config.Testing == true) {
		logger.Warn("WARNING: Neither API server nor worker running!")
		logger.Info("If this is uninteded please check configuration:",
			zap.Bool("Testing", config.Testing),
			zap.Bool("Server.Enable", config.Server.Enable),
			zap.Bool("Worker.Enable", config.Worker.Enable),
		)
		logger.Info("FYI: Setting `Testing` to `true` always disables the worker!")
	}
	quit := make(chan os.Signal)
	signal.Notify(quit, os.Interrupt)
	<-quit

	wrk.Shutdown()
	apiServer.Shutdown()
}
