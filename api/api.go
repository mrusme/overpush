package api

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/cors"
	"github.com/gofiber/fiber/v3/middleware/healthcheck"
	"github.com/gofiber/fiber/v3/middleware/limiter"
	"github.com/gofiber/fiber/v3/middleware/requestid"
	"github.com/gofiber/storage/redis/v3"
	"github.com/hibiken/asynq"
	"github.com/mrusme/overpush/config"
	"github.com/mrusme/overpush/database"
	"github.com/mrusme/overpush/fiberzap"
	"github.com/mrusme/overpush/repositories"
	"go.uber.org/zap"
)

type API struct {
	cfg   *config.Config
	log   *zap.Logger
	repos *repositories.Repositories
	app   *fiber.App
	redis *asynq.Client
}

func New(
	cfg *config.Config,
	log *zap.Logger,
) (*API, error) {
	api := new(API)

	api.cfg = cfg
	api.log = log

	if api.cfg.Server.Enable == true {
		api.app = fiber.New(fiber.Config{
			StrictRouting:      false,
			CaseSensitive:      false,
			BodyLimit:          api.cfg.Server.BodyLimit,
			Concurrency:        api.cfg.Server.Concurrency,
			ProxyHeader:        api.cfg.Server.ProxyHeader,
			EnableIPValidation: api.cfg.Server.EnableIPValidation,
			TrustProxy:         api.cfg.Server.TrustProxy,
			TrustProxyConfig: fiber.TrustProxyConfig{
				Loopback: api.cfg.Server.TrustLoopback,
				Proxies:  api.cfg.Server.TrustProxies,
			},
			ReduceMemoryUsage: api.cfg.Server.ReduceMemoryUsage,
			ServerHeader:      api.cfg.Server.ServerHeader,
			AppName:           "overpush",
			ErrorHandler: func(c fiber.Ctx, err error) error {
				return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
					"errors":  []string{err.Error()},
					"status":  0,
					"request": requestid.FromContext(c),
				})
			},
		})
	}

	return api, nil
}

func (api *API) LoadMiddlewares() error {
	if api.cfg.Server.Enable == false {
		api.log.Info("Server not enabled",
			zap.Bool("Server.Enable", api.cfg.Server.Enable),
		)
		return nil
	}

	api.app.Use(fiberzap.New(fiberzap.Config{
		Logger: api.log,
	}))
	api.app.Use(requestid.New())
	api.app.Use(cors.New())

	api.app.Get(fmt.Sprintf("/_internal/health%s", healthcheck.LivenessEndpoint),
		healthcheck.New())
	api.app.Get(fmt.Sprintf("/_internal/health%s", healthcheck.ReadinessEndpoint),
		healthcheck.New())
	api.app.Get(fmt.Sprintf("/_internal/health%s", healthcheck.StartupEndpoint),
		healthcheck.New())

	limiterCfg := limiter.Config{
		Next: func(c fiber.Ctx) bool {
			return c.IP() == "127.0.0.1"
		},
		Max: api.cfg.Server.Limiter.MaxReqests,
		Expiration: time.Second *
			time.Duration(api.cfg.Server.Limiter.PerDurationInSeconds),
		SkipFailedRequests:     api.cfg.Server.Limiter.IgnoreFailedRequests,
		SkipSuccessfulRequests: false,
		KeyGenerator: func(c fiber.Ctx) string {
			return fmt.Sprintf(
				"%s-%s",
				c.Get("x-forwarded-for"),
				c.Params("token"),
			)
		},
		LimitReached: func(c fiber.Ctx) error {
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
				"errors":  []string{"Slow down, cowboy!"},
				"status":  0,
				"request": requestid.FromContext(c),
			})
		},
	}
	if api.cfg.Server.Limiter.UseRedis == true {
		var redisStorage *redis.Storage
		conn := strings.SplitN(api.cfg.Redis.Connection, ":", 2)
		if len(conn) != 2 {
			return errors.New("Could not parse Redis.Connection into HOST:PORT for Limiter")
		}
		host := conn[0]
		port, err := strconv.Atoi(conn[1])
		if err != nil {
			return err
		}

		if api.cfg.Testing == false {
			if api.cfg.Redis.Cluster == false {
				if api.cfg.Redis.Failover == false {
					redisStorage = redis.New(redis.Config{
						Host:     host,
						Port:     port,
						Username: api.cfg.Redis.Username,
						Password: api.cfg.Redis.Password,
					})
				} else {
					redisStorage = redis.New(redis.Config{
						MasterName: api.cfg.Redis.MasterName,
						Addrs:      api.cfg.Redis.Connections,
						Username:   api.cfg.Redis.Username,
						Password:   api.cfg.Redis.Password,
					})
				}
			} else {
				redisStorage = redis.New(redis.Config{
					Addrs:    api.cfg.Redis.Connections,
					Username: api.cfg.Redis.Username,
					Password: api.cfg.Redis.Password,
				})
			}
		}

		limiterCfg.Storage = redisStorage
	}
	api.app.Use(limiter.New(limiterCfg))

	return nil
}

func (api *API) AttachRoutes() {
	if api.cfg.Server.Enable == false {
		api.log.Info("Server not enabled",
			zap.Bool("Server.Enable", api.cfg.Server.Enable),
		)
		return
	}

	api.app.Post("/1/messages.json", handler(api))
	api.app.Post("/:token", handler(api))
	api.app.Post("/_internal/submit/:token", handler(api))
}

func (api *API) Run() error {
	var err error

	if api.cfg.Server.Enable == false {
		api.log.Info("Server not enabled",
			zap.Bool("Server.Enable", api.cfg.Server.Enable),
		)
		return nil
	}

	if api.cfg.Testing == false {
		if api.cfg.Redis.Cluster == false {
			if api.cfg.Redis.Failover == false {
				api.redis = asynq.NewClient(asynq.RedisClientOpt{
					Addr:     api.cfg.Redis.Connection,
					Username: api.cfg.Redis.Username,
					Password: api.cfg.Redis.Password,
				})
			} else {
				api.redis = asynq.NewClient(asynq.RedisFailoverClientOpt{
					MasterName:    api.cfg.Redis.MasterName,
					SentinelAddrs: api.cfg.Redis.Connections,
					Username:      api.cfg.Redis.Username,
					Password:      api.cfg.Redis.Password,
				})
			}
		} else {
			api.redis = asynq.NewClient(asynq.RedisClusterClientOpt{
				Addrs:    api.cfg.Redis.Connections,
				Username: api.cfg.Redis.Username,
				Password: api.cfg.Redis.Password,
			})
		}
		defer api.redis.Close()
	}

	var db *database.Database
	if db, err = database.New(api.cfg, api.log); err != nil {
		return(err)
	}

	var repos *repositories.Repositories
	if repos, err = repositories.New(api.cfg, db); err != nil {
		db.Shutdown()
		return(err)
	}
	api.repos = repos

	functionName := os.Getenv("AWS_LAMBDA_FUNCTION_NAME")

	if functionName == "" {
		listenAddr := fmt.Sprintf(
			"%s:%s",
			api.cfg.Server.BindIP,
			api.cfg.Server.Port,
		)
		if err := api.app.Listen(listenAddr); err != nil && err != http.ErrServerClosed {
			api.log.Fatal(
				"Server failed",
				zap.Error(err),
			)
		}
	} else {
		lambda.Start(api.AWSLambdaHandler)
	}

	return nil
}

func (api *API) Shutdown() error {
	if api.cfg.Server.Enable == false {
		api.log.Info("Server not enabled",
			zap.Bool("Server.Enable", api.cfg.Server.Enable),
		)
		return nil
	}
	api.app.ShutdownWithTimeout(time.Second * 5)
	api.repos.Shutdown()
	return nil
}
