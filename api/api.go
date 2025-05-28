package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/Jeffail/gabs/v2"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/cors"
	"github.com/gofiber/fiber/v3/middleware/requestid"
	"github.com/hibiken/asynq"
	"github.com/markusmobius/go-dateparser"
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
		StrictRouting: false,
		CaseSensitive: false,
		Concurrency:   256 * 1024, // TODO: Make configurable
		ProxyHeader:   "",         // TODO: Make configurable
		// EnableTrustedProxyCheck: false,      // TODO: Make configurable
		// TrustedProxies:          []string{}, // TODO: Make configurable
		ReduceMemoryUsage: false,      // TODO: Make configurable
		ServerHeader:      "AmazonS3", // Let's distract script kiddies
		AppName:           "overpush",
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

	handler := func(c fiber.Ctx) error {
		var user string
		var token string
		var msg *messages.Request
		var appFormat string
		var application lib.Application
		var err error

		bound := c.Bind()

		api.log.Debug("Received request, processing ...")

		if c.Route().Path == "/1/messages.json" {
			appFormat = "pushover"
		} else {
			token = c.Params("token")
			user, err = api.cfg.GetUserKeyFromToken(token)
			if err != nil {
				return c.Status(fiber.ErrBadRequest.Code).JSON(fiber.Map{
					"error":   err.Error(),
					"status":  0,
					"request": requestid.FromContext(c),
				})
			}

			api.log.Debug("Retrieving application ...")
			application, err = api.cfg.GetApplication(user, token)
			if err != nil {
				return c.Status(fiber.ErrBadRequest.Code).JSON(fiber.Map{
					"error":   err.Error(),
					"status":  0,
					"request": requestid.FromContext(c),
				})
			}

			appFormat = application.Format
		}

		if appFormat == "pushover" {
			api.log.Debug("Application is pushover, processing ...")
			req := new(messages.Request)

			if err := bound.Body(req); err != nil {
				return c.Status(fiber.ErrBadRequest.Code).JSON(fiber.Map{
					"error":   err.Error(),
					"status":  0,
					"request": requestid.FromContext(c),
				})
			}

			if err := validate.Struct(req); err != nil {
				return c.Status(fiber.ErrBadRequest.Code).JSON(fiber.Map{
					"error":   err.Error(),
					"status":  0,
					"request": requestid.FromContext(c),
				})
			}

			msg = req
		} else if appFormat == "custom" {
			api.log.Debug("Application is custom, processing ...")
			req := make(map[string]interface{})

			if err := bound.Body(&req); err != nil {
				api.log.Error("Error parsing", zap.Error(err))
				return c.Status(fiber.ErrBadRequest.Code).JSON(fiber.Map{
					"error":   err.Error(),
					"status":  0,
					"request": requestid.FromContext(c),
				})
			}

			locations := make(map[string]*gabs.Container)
			locations["body"] = gabs.Wrap(req)
			msg = new(messages.Request)
			var found bool
			var tmp string

			msg.Attachment, found = application.CustomFormat.
				GetValue(locations, application.CustomFormat.Attachment)

			msg.AttachmentBase64, found = application.CustomFormat.
				GetValue(locations, application.CustomFormat.AttachmentBase64)

			msg.AttachmentType, found = application.CustomFormat.
				GetValue(locations, application.CustomFormat.AttachmentType)

			msg.Device, found = application.CustomFormat.
				GetValue(locations, application.CustomFormat.Device)

			tmp, found = application.CustomFormat.
				GetValue(locations, application.CustomFormat.HTML)
			if found {
				if tmp == "0" {
					msg.HTML = 0
				} else if tmp == "1" {
					msg.HTML = 1
				}
			}

			msg.Message, found = application.CustomFormat.
				GetValue(locations, application.CustomFormat.Message)

			tmp, found = application.CustomFormat.
				GetValue(locations, application.CustomFormat.Priority)
			if found {
				msg.Priority, _ = strconv.Atoi(tmp)
				if msg.Priority < -2 || msg.Priority > 2 {
					msg.Priority = 0
				}
			}

			tmp, found = application.CustomFormat.
				GetValue(locations, application.CustomFormat.TTL)
			if found {
				msg.TTL, _ = strconv.Atoi(tmp)
			}

			tmp, found = application.CustomFormat.
				GetValue(locations, application.CustomFormat.Timestamp)
			if found {
				dt, err := dateparser.Parse(nil, tmp)
				if err == nil {
					msg.Timestamp = dt.Time.Unix()
				}
			}

			msg.Title, found = application.CustomFormat.
				GetValue(locations, application.CustomFormat.Title)

			msg.URL, found = application.CustomFormat.
				GetValue(locations, application.CustomFormat.URL)

			msg.URLTitle, found = application.CustomFormat.
				GetValue(locations, application.CustomFormat.URLTitle)

		}

		msg.User = user
		msg.Token = token

		api.log.Debug("Validating request...")
		if err := validate.Struct(msg); err != nil {
			api.log.Error("Error validating", zap.Error(err))
			return c.Status(fiber.ErrBadRequest.Code).JSON(fiber.Map{
				"error":   err.Error(),
				"status":  0,
				"request": requestid.FromContext(c),
			})
		}

		payload, err := json.Marshal(msg)
		if err != nil {
			return c.Status(fiber.ErrBadRequest.Code).JSON(fiber.Map{
				"error":   err.Error(),
				"status":  0,
				"request": requestid.FromContext(c),
			})
		}

		if api.cfg.Testing == false {
			api.log.Debug("Enqueueing request", zap.ByteString("payload", payload))
			_, err = api.redis.Enqueue(asynq.NewTask("message", payload))
			if err != nil {
				return c.Status(fiber.ErrInternalServerError.Code).JSON(fiber.Map{
					"error":   err.Error(),
					"status":  0,
					"request": requestid.FromContext(c),
				})
			}
		} else {
			api.log.Debug("Not enqueueing request", zap.ByteString("payload", payload))
		}

		return c.JSON(fiber.Map{
			"status":  1,
			"request": requestid.FromContext(c),
		})
	}

	api.app.Post("/1/messages.json", handler)
	api.app.Post("/:token", handler)
}

func (api *API) Run() error {
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
