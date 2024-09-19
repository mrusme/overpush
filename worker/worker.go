package worker

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/hibiken/asynq"
	"github.com/mrusme/overpush/api/messages"
	"github.com/mrusme/overpush/lib"
	"github.com/xmppo/go-xmpp"
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
	for _, user := range wrk.cfg.Users {
		if user.Key == m.User {
			for _, app := range user.Applications {
				if app.Token == m.Token {
					for _, target := range wrk.cfg.Targets {
						if app.Target == target.ID {
							switch target.Type {
							case "xmpp":
								execErr = wrk.ExecuteXMPP(target.Args, m)
							}
						}
					}
				}
			}
		}
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
