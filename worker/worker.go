package worker

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/hibiken/asynq"
	"github.com/mrusme/overpush/api/messages"
	"github.com/mrusme/overpush/config"
	"github.com/mrusme/overpush/repositories"
	"github.com/xmppo/go-xmpp"
	"go.uber.org/zap"
)

type Worker struct {
	cfg      *config.Config
	log      *zap.Logger
	repos    *repositories.Repositories
	redis    *asynq.Server
	redisMux *asynq.ServeMux
}

func New(
	cfg *config.Config,
	log *zap.Logger,
	repos *repositories.Repositories,
) (*Worker, error) {
	wrk := new(Worker)

	wrk.cfg = cfg
	wrk.log = log
	wrk.repos = repos
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
	return nil
}

func (wrk *Worker) HandleMessage(ctx context.Context, t *asynq.Task) error {
	var m messages.Request
	if err := json.Unmarshal(t.Payload(), &m); err != nil {
		return err
	}

	wrk.log.Debug("Working on message", zap.ByteString("payload", t.Payload()))

	var execErr error

	targetID, err := wrk.repos.Target.GetTargetID(m.User, m.Token)
	if err != nil {
		return err
	}

	target, err := wrk.repos.Target.GetTargetByID(targetID)
	if err != nil {
		return err
	}

	switch target.Type {
	case "xmpp":
		execErr = wrk.ExecuteXMPP(target.Args, m)
	case "apprise":
		execErr = wrk.ExecuteApprise(target.Args, m)
	}

	return execErr
}

func (wrk *Worker) ExecuteXMPP(args map[string]string, m messages.Request) error {
	var jabber *xmpp.Client

	xmppServer := args["server"]
	xmppTLS, err := strconv.ParseBool(args["tls"])
	if err != nil {
		xmppTLS = true
	}
	xmppUsername := args["username"]
	xmppPassword := args["password"]
	destinationUsername := args["destination"]

	xmpp.DefaultConfig = &tls.Config{
		ServerName:         strings.Split(xmppServer, ":")[0],
		InsecureSkipVerify: false,
	}

	jabberOpts := xmpp.Options{
		Host:          xmppServer,
		User:          xmppUsername,
		Password:      xmppPassword,
		NoTLS:         !xmppTLS,
		Debug:         false,
		Session:       true,
		Status:        "xa",
		StatusMessage: "Pushing over ...",
	}

	jabber, err = jabberOpts.NewClient()
	if err != nil {
		fmt.Println(err)
		return err
	}
	defer jabber.Close()

	_, err = jabber.Send(xmpp.Chat{
		Remote: destinationUsername,
		Type:   "chat",
		Text:   m.ToString(),
	})
	if err != nil {
		fmt.Println(err)
		return err
	}

	return nil
}

func (wrk *Worker) ExecuteApprise(args map[string]string, m messages.Request) error {
	ctx, _ := context.WithTimeout(context.Background(), 10*time.Second)
	cmd := exec.CommandContext(
		ctx,
		"python",
		args["apprise"],
		"-vv",
		"-t", m.Title,
		"-b", m.Message,
		args["connection"],
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return err
	}

	if err := cmd.Wait(); err != nil {
		return err
	}

	return nil
}
