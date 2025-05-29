package main

import (
	"os"
	"os/signal"

	"github.com/mrusme/overpush/api"
	"github.com/mrusme/overpush/config"
	"github.com/mrusme/overpush/database"
	"github.com/mrusme/overpush/repositories"
	"github.com/mrusme/overpush/repositories/user"
	"github.com/mrusme/overpush/worker"

	"go.uber.org/zap"
)

func init() {
}

func main() {
	var logger *zap.Logger
	var err error

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

	var db *database.Database
	if db, err = database.New(&config, logger); err != nil {
		panic(err)
	}

	var userRepo *user.Repository
	if userRepo, err = user.New(&config, db); err != nil {
		db.Shutdown()
		panic(err)
	}

	var repos repositories.Repositories
	repos.User = userRepo

	var wrk *worker.Worker
	if wrk, err = worker.New(&config, logger, repos); err != nil {
		db.Shutdown()
		panic(err)
	}
	go wrk.Run()

	var apiServer *api.API
	if apiServer, err = api.New(&config, logger, repos); err != nil {
		wrk.Shutdown()
		db.Shutdown()
		panic(err)
	}
	if err = apiServer.LoadMiddlewares(); err != nil {
		wrk.Shutdown()
		db.Shutdown()
		panic(err)
	}
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
	db.Shutdown()
}
