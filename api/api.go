package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/cors"
	"github.com/gofiber/fiber/v3/middleware/requestid"
	"github.com/hibiken/asynq"
	"github.com/mrusme/overpush/api/messages"
	"github.com/mrusme/overpush/fiberzap"
	"github.com/mrusme/overpush/lib"
	"go.uber.org/zap"

	fiberadapter "github.com/mrusme/overpush/fiberadapter"
)

type API struct {
	cfg   *lib.Config
	log   *zap.Logger
	app   *fiber.App
	redis *asynq.Client
}

func New(cfg *lib.Config, log *zap.Logger) (*API, error) {
	api := new(API)

	api.cfg = cfg
	api.log = log

	api.app = fiber.New(fiber.Config{
		StrictRouting:           false,
		CaseSensitive:           false,
		Concurrency:             256 * 1024, // TODO: Make configurable
		ProxyHeader:             "",         // TODO: Make configurable
		EnableTrustedProxyCheck: false,      // TODO: Make configurable
		TrustedProxies:          []string{}, // TODO: Make configurable
		ReduceMemoryUsage:       false,      // TODO: Make configurable
		ServerHeader:            "AmazonS3", // Let's distract script kiddies
		AppName:                 "overpush",
		ErrorHandler: func(c fiber.Ctx, err error) error {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"errors":  []string{err.Error()},
				"status":  0,
				"request": requestid.FromContext(c),
			})
		},
	})
	api.app.Use(fiberzap.New(fiberzap.Config{
		Logger: api.log,
	}))
	api.app.Use(requestid.New())
	api.app.Use(cors.New())
	api.attachRoutes()

	return api, nil
}

func (api *API) AWSLambdaHandler(
	ctx context.Context,
	req events.APIGatewayProxyRequest,
) (events.APIGatewayProxyResponse, error) {
	var fiberLambda *fiberadapter.FiberLambda
	fiberLambda = fiberadapter.New(api.app)
	return fiberLambda.ProxyWithContext(ctx, req)
}

func (api *API) attachRoutes() {
	validate := validator.New(validator.WithRequiredStructEnabled())

	api.app.Post("/1/messages.json", func(c fiber.Ctx) error {
		req := new(messages.Request)

		bound := c.Bind()

		if err := bound.Body(req); err != nil {
			return c.JSON(fiber.Map{
				"error":   err.Error(),
				"status":  0,
				"request": requestid.FromContext(c),
			})
		}

		if err := validate.Struct(req); err != nil {
			return c.JSON(fiber.Map{
				"error":   err.Error(),
				"status":  0,
				"request": requestid.FromContext(c),
			})
		}

		payload, err := json.Marshal(req)
		if err != nil {
			return c.JSON(fiber.Map{
				"error":   err.Error(),
				"status":  0,
				"request": requestid.FromContext(c),
			})
		}

		api.log.Debug("Enqueueing request", zap.ByteString("payload", payload))
		_, err = api.redis.Enqueue(asynq.NewTask("message", payload))
		if err != nil {
			return c.JSON(fiber.Map{
				"error":   err.Error(),
				"status":  0,
				"request": requestid.FromContext(c),
			})
		}

		return c.JSON(fiber.Map{
			"status":  1,
			"request": requestid.FromContext(c),
		})
	})
}

func (api *API) Run() error {
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
	api.app.ShutdownWithTimeout(time.Second * 5)
	return nil
}
