package worker

import (
	"context"
	"encoding/json"

	"github.com/hibiken/asynq"
	"github.com/mrusme/overpush/api/messages"
	"github.com/mrusme/overpush/lib"
	"go.uber.org/zap"
)

type Worker struct {
	cfg      *lib.Config
	log      *zap.Logger
	redis    *asynq.Server
	redisMux *asynq.ServeMux
}

func New(cfg *lib.Config, log *zap.Logger) (*Worker, error) {
	wrk := new(Worker)

	wrk.cfg = cfg
	wrk.log = log
	return wrk, nil
}

func (wrk *Worker) Run() {
	wrk.redis = asynq.NewServer(
		asynq.RedisClientOpt{
			Addr:     wrk.cfg.Redis.Connection,
			Username: wrk.cfg.Redis.Username,
			Password: wrk.cfg.Redis.Password,
		},
		asynq.Config{
			// Logger:      wrk.log,
			Concurrency: 1,
		},
	)

	wrk.redisMux = asynq.NewServeMux()
	wrk.redisMux.HandleFunc("message", wrk.HandleMessage)

	if err := wrk.redis.Run(wrk.redisMux); err != nil {
		wrk.log.Fatal("Worker failed", zap.Error(err))
	}
}

func (wrk *Worker) HandleMessage(ctx context.Context, t *asynq.Task) error {
	var m messages.Request
	if err := json.Unmarshal(t.Payload(), &m); err != nil {
		return err
	}

	wrk.log.Debug("Working on message", zap.ByteString("payload", t.Payload()))
	return nil
}
