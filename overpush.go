package main

import (
	"context"
	"fmt"
	"os"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	fiberadapter "github.com/mrusme/overpush/fiberadapter"
	"github.com/mrusme/overpush/fiberzap"

	"go.uber.org/zap"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/requestid"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
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
	})
	fiberApp.Use(fiberzap.New(fiberzap.Config{
		Logger: logger,
	}))
	fiberApp.Use(requestid.New())

	fiberApp.Post("/1/messages.json", func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"status":  1,
			"request": requestid.FromContext(c),
		})
	})

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
