package main

import (
	"context"
	"fmt"
	"os"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/hibiken/asynq"
	"github.com/mrusme/overpush/api/messages"
	fiberadapter "github.com/mrusme/overpush/fiberadapter"
	"github.com/mrusme/overpush/fiberzap"

	"go.uber.org/zap"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/cors"
	"github.com/gofiber/fiber/v3/middleware/requestid"
)

var fiberApp *fiber.App
var fiberLambda *fiberadapter.FiberLambda

var logger *zap.Logger

func init() {
	fiberLambda = fiberadapter.New(fiberApp)

	// if config.Debug == "true" {
	logger, _ = zap.NewDevelopment()
	// } else {
	// logger, _ = zap.NewProduction()
	// }
	defer logger.Sync()
	// TODO: Use sugarLogger
	// sugar := logger.Sugar()
}

func AWSLambdaHandler(
	ctx context.Context,
	req events.APIGatewayProxyRequest,
) (events.APIGatewayProxyResponse, error) {
	return fiberLambda.ProxyWithContext(ctx, req)
}

func main() {
	ac := asynq.NewClient(asynq.RedisClientOpt{
		Addr:     os.Getenv("REDIS"),
		Username: os.Getenv("REDIS_USERNAME"),
		Password: os.Getenv("REDIS_PASSWORD"),
	})
	defer ac.Close()

	// var err error
	fiberApp := fiber.New(fiber.Config{
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
	fiberApp.Use(fiberzap.New(fiberzap.Config{
		Logger: logger,
	}))
	fiberApp.Use(requestid.New())
	fiberApp.Use(cors.New())
	messages.New(fiberApp, ac)

	functionName := os.Getenv("AWS_LAMBDA_FUNCTION_NAME")

	if functionName == "" {
		listenAddr := fmt.Sprintf(
			"%s:%s",
			// config.Server.BindIP,
			// config.Server.Port,
			"127.0.0.1",
			"8080",
		)
		logger.Fatal(
			"Server failed",
			zap.Error(fiberApp.Listen(listenAddr)),
		)
	} else {
		lambda.Start(AWSLambdaHandler)
	}
}
