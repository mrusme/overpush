// Originally from https://gl.oddhunters.com/pub/fiberzap
// Copyright (apparently) by Ozgur Boru <boruozgur@yandex.com.tr>
// and "mert" (https://gl.oddhunters.com/mert)
// Updated for Fiber v3 and reduced to non-sensitive fields by github.com/mrusme
package fiberzap

import (
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/requestid"
	"go.uber.org/zap"
)

// Config defines the config for middleware
type Config struct {
	// Next defines a function to skip this middleware when returned true.
	//
	// Optional. Default: nil
	Next func(c fiber.Ctx) bool

	// Logger defines zap logger instance
	Logger *zap.Logger
}

// New creates a new middleware handler
func New(config ...Config) fiber.Handler {
	cfg := config[0]

	return func(c fiber.Ctx) error {
		if cfg.Next != nil && cfg.Next(c) {
			return c.Next()
		}

		start := time.Now()

		chainErr := c.Next()

		if chainErr != nil {
			if err := c.App().Config().ErrorHandler(c, chainErr); err != nil {
				_ = c.SendStatus(fiber.StatusInternalServerError)
			}
		}

		fields := []zap.Field{
			zap.String("method", c.Method()),
			zap.String("route", c.Route().Path),
			zap.Int("status", c.Response().StatusCode()),
			zap.Duration("duration", time.Since(start)),
			zap.String("request", requestid.FromContext(c)),
		}

		if chainErr != nil {
			cfg.Logger.Error(chainErr.Error(), fields...)
			return nil
		}

		cfg.Logger.Info("api.request", fields...)

		return nil
	}
}
