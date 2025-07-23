package worker

import (
	"bytes"
	"context"
	"encoding/json"
	"io"

	"filippo.io/age"
	"github.com/hibiken/asynq"
	"github.com/mrusme/overpush/config"
	"github.com/mrusme/overpush/database"
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
) (*Worker, error) {
	wrk := new(Worker)

	wrk.cfg = cfg
	wrk.log = log

	return wrk, nil
}

func (wrk *Worker) Run() error {
	var err error
	var db *database.Database
	var repos *repositories.Repositories

	if (wrk.cfg.Worker.Enable == false && wrk.cfg.Testing == true) ||
		(wrk.cfg.Worker.Enable == true) {
		wrk.log.Info("Worker initializing db and repos",
			zap.Bool("Testing", wrk.cfg.Testing),
			zap.Bool("Worker.Enable", wrk.cfg.Worker.Enable),
		)

		if db, err = database.New(wrk.cfg, wrk.log); err != nil {
			return (err)
		}

		if repos, err = repositories.New(wrk.cfg, db); err != nil {
			db.Shutdown()
			return (err)
		}
		wrk.repos = repos
	}

	if wrk.cfg.Worker.Enable == false || wrk.cfg.Testing == true {
		wrk.log.Info("Worker not enabled",
			zap.Bool("Testing", wrk.cfg.Testing),
			zap.Bool("Worker.Enable", wrk.cfg.Worker.Enable),
		)
		return nil
	}

	targetCfgs, err := wrk.repos.Target.GetTargets()
	if err != nil {
		db.Shutdown()
		return err
	}

	if wrk.ts, err = targets.New(wrk.cfg, wrk.log, targetCfgs); err != nil {
		db.Shutdown()
		return err
	}

	if err := wrk.ts.LoadAll(); err != nil {
		wrk.log.Fatal("Worker failed to load targets", zap.Error(err))
		db.Shutdown()
		return err
	}

	if err := wrk.ts.RunAll(); err != nil {
		wrk.log.Fatal("Worker failed to run targets", zap.Error(err))
		db.Shutdown()
		return err
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
	wrk.redisMux.HandleFunc("message", asynqHandler(wrk))

	if err := wrk.redis.Run(wrk.redisMux); err != nil {
		wrk.log.Fatal("Worker failed", zap.Error(err))
	}
	return err
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

	wrk.repos.Shutdown()
	return nil
}

func asynqHandler(wrk *Worker) func(context.Context, *asynq.Task) error {
	return wrk.HandleMessage
}

func (wrk *Worker) HandleMessage(ctx context.Context, t *asynq.Task) error {
	var m message.Message
	if err := json.Unmarshal(t.Payload(), &m); err != nil {
		return err
	}

	wrk.log.Debug("Working on message", zap.ByteString("payload", t.Payload()))

	app, err := wrk.repos.Application.GetApplication(m.User, m.Token)
	if err != nil {
		wrk.log.Debug("Worker encountered error for User.GetApplication",
			zap.Error(err))
		// Note: Even though this error indicates that the user application was not
		// found, e.g. because the user deleted it in the meantime, it is still
		// worth to retry the message, if this came up due to a database related
		// issue.
		return err
	}
	if m.IsViaSubmit() == false && app.Enable == false {
		wrk.log.Debug("Worker disregarding job, application not enabled",
			zap.String("Application.Token", app.Token))
		return nil
	}

	target, err := wrk.repos.Target.GetTargetByID(app.Target)
	if err != nil {
		wrk.log.Debug("Worker encountered error for Target.GetTargetByID",
			zap.Error(err))
		// Note: Even though this error indicates that the user target was not
		// found, e.g. because the user changed it in the meantime, it is still
		// worth to retry the message, if this came up due to a database related
		// issue.
		return err
	}
	if m.IsViaSubmit() == false && target.Enable == false {
		wrk.log.Debug("Worker disregarding job, target not enabled",
			zap.String("Target.ID", target.ID))
		return nil
	}

	// In the config we only store a single target's args in
	// Application.TargetArgs; In the Database we store per-target configs:
	// { "target1-uuid": { // config }, "target2-uuid": { // config } }
	realTarget, ok := app.TargetArgs[target.ID]
	if ok {
		app.TargetArgs = realTarget.(map[string]interface{})
	}

	wrk.log.Debug("Worker checking encryption requirements",
		zap.String("EncryptionType", app.EncryptionType))
	if app.EncryptionType == "age" {
		if err = wrk.EncryptWithAge(
			&m,
			app.EncryptionRecipients,
			app.EncryptTitle,
			app.EncryptMessage,
			app.EncryptAttachment); err != nil {
			wrk.log.Error("Worker encryption failed",
				zap.Error(err))
			return err
		}
	}

	wrk.log.Debug("Worker executing target",
		zap.String("Target.Type", target.Type),
		zap.Any("Target.Args", target.Args),
		zap.Any("Application.TargetArgs", app.TargetArgs),
	)

	if err := wrk.ts.Execute(
		target.ID,
		m,
		app.TargetArgs,
	); err != nil {
		wrk.log.Debug("Worker target execution failed",
			zap.Error(err))
		return err
	}

	if m.IsViaSubmit() == false {
		if err = wrk.repos.Application.IncrementStat(
			"No need when DB",
			app.Token,
			"sent",
		); err != nil {
			wrk.log.Error("Application stat not increased",
				zap.String("stat", "sent"),
				zap.Error(err))
		}
	}

	return nil
}

func (wrk *Worker) EncryptWithAge(
	m *message.Message,
	recipients []string,
	title bool,
	message bool,
	attachment bool,
) error {
	if title == false && message == false == attachment == false {
		return nil
	}

	var rcpts []age.Recipient
	for _, recipient := range recipients {
		rcpt, err := age.ParseX25519Recipient(recipient)
		if err != nil {
			return err
		}
		rcpts = append(rcpts, rcpt)
	}

	if title == true {
		out := &bytes.Buffer{}
		w, err := age.Encrypt(out, rcpts...)
		if err != nil {
			return err
		}
		if _, err := io.WriteString(w, m.Title); err != nil {
			return err
		}
		if err := w.Close(); err != nil {
			return err
		}
		m.Title = out.String()
	}

	if message == true {
		out := &bytes.Buffer{}
		w, err := age.Encrypt(out, rcpts...)
		if err != nil {
			return err
		}
		if _, err := io.WriteString(w, m.Message); err != nil {
			return err
		}
		if err := w.Close(); err != nil {
			return err
		}
		m.Message = out.String()
	}

	if attachment == true {
		out := &bytes.Buffer{}
		w, err := age.Encrypt(out, rcpts...)
		if err != nil {
			return err
		}
		if len(m.Attachment) > 0 {
			if _, err := io.WriteString(w, m.Attachment); err != nil {
				return err
			}
		} else if len(m.AttachmentBase64) > 0 && len(m.AttachmentType) > 0 {
			if _, err := io.WriteString(w, m.AttachmentBase64); err != nil {
				return err
			}
		}

		if err := w.Close(); err != nil {
			return err
		}
		if len(m.Attachment) > 0 {
			m.Attachment = out.String()
		} else if len(m.AttachmentBase64) > 0 && len(m.AttachmentType) > 0 {
			m.AttachmentBase64 = out.String()
		}
	}

	return nil
}
