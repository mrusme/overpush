package worker

import (
	"context"
	"encoding/json"

	"github.com/hibiken/asynq"
	"github.com/mrusme/overpush/config"
	"github.com/mrusme/overpush/helpers"
	"github.com/mrusme/overpush/models/message"
	"github.com/mrusme/overpush/repositories"
	"github.com/mrusme/overpush/worker/targets"
	"go.uber.org/zap"
)

type Worker struct {
	cfg      *config.Config
	log      *zap.Logger
	repos    *repositories.Repositories
	ts       *targets.Targets
	redis    *asynq.Server
	redisMux *asynq.ServeMux
}

func New(
	cfg *config.Config,
	log *zap.Logger,
	repos *repositories.Repositories,
) (*Worker, error) {
	var err error

	wrk := new(Worker)

	wrk.cfg = cfg
	wrk.log = log
	wrk.repos = repos

	if wrk.ts, err = targets.New(wrk.cfg, wrk.log); err != nil {
		return nil, err
	}

	return wrk, nil
}

func (wrk *Worker) Run() {
	if wrk.cfg.Worker.Enable == false || wrk.cfg.Testing == true {
		wrk.log.Info("Worker not enabled",
			zap.Bool("Testing", wrk.cfg.Testing),
			zap.Bool("Worker.Enable", wrk.cfg.Worker.Enable),
		)
		return
	}

	if err := wrk.ts.RunAll(); err != nil {
		wrk.log.Fatal("Worker failed to run targets", zap.Error(err))
	}

	if wrk.cfg.Redis.Cluster == false {
		if wrk.cfg.Redis.Failover == false {
			wrk.redis = asynq.NewServer(
				asynq.RedisClientOpt{
					Addr:     wrk.cfg.Redis.Connection,
					Username: wrk.cfg.Redis.Username,
					Password: wrk.cfg.Redis.Password,
				},
				asynq.Config{
					Logger:      wrk.log.Sugar(),
					Concurrency: wrk.cfg.Redis.Concurrency,
				},
			)
		} else {
			wrk.redis = asynq.NewServer(
				asynq.RedisFailoverClientOpt{
					MasterName:    wrk.cfg.Redis.MasterName,
					SentinelAddrs: wrk.cfg.Redis.Connections,
					Username:      wrk.cfg.Redis.Username,
					Password:      wrk.cfg.Redis.Password,
				},
				asynq.Config{
					Logger:      wrk.log.Sugar(),
					Concurrency: wrk.cfg.Redis.Concurrency,
				},
			)
		}
	} else {
		wrk.redis = asynq.NewServer(
			asynq.RedisClusterClientOpt{
				Addrs:    wrk.cfg.Redis.Connections,
				Username: wrk.cfg.Redis.Username,
				Password: wrk.cfg.Redis.Password,
			},
			asynq.Config{
				Logger:      wrk.log.Sugar(),
				Concurrency: wrk.cfg.Redis.Concurrency,
			},
		)
	}

	wrk.redisMux = asynq.NewServeMux()
	wrk.redisMux.HandleFunc("message", wrk.HandleMessage)

	if err := wrk.redis.Run(wrk.redisMux); err != nil {
		wrk.log.Fatal("Worker failed", zap.Error(err))
	}
}

func (wrk *Worker) Shutdown() error {
	if wrk.cfg.Worker.Enable == false || wrk.cfg.Testing == true {
		wrk.log.Info("Worker not enabled",
			zap.Bool("Testing", wrk.cfg.Testing),
			zap.Bool("Worker.Enable", wrk.cfg.Worker.Enable),
		)
		return nil
	}

	wrk.redis.Shutdown()

	if ok, errs := wrk.ts.ShutdownAll(); !ok {
		err := helpers.ErrorsToError(errs)
		wrk.log.Error("Worker shutdown with target errors",
			zap.Error(err))
	}
	return nil
}

func (wrk *Worker) HandleMessage(ctx context.Context, t *asynq.Task) error {
	var m message.Message
	if err := json.Unmarshal(t.Payload(), &m); err != nil {
		return err
	}

	wrk.log.Debug("Working on message", zap.ByteString("payload", t.Payload()))

	var execErr error

	app, err := wrk.repos.User.GetApplication(m.User, m.Token)
	if err != nil {
		return err
	}
	if m.IsViaSubmit() == false && app.Enable == false {
		wrk.log.Debug("Worker disregarding job, application not enabled",
			zap.String("Application.Token", app.Token))
		return nil
	}

	target, err := wrk.repos.Target.GetTargetByID(app.Target)
	if err != nil {
		return err
	}
	if m.IsViaSubmit() == false && target.Enable == false {
		wrk.log.Debug("Worker disregarding job, target not enabled",
			zap.String("Target.ID", target.ID))
		return nil
	}

	wrk.ts.Execute(target.Type, m, target.Args, app.TargetArgs)

	return execErr
}
