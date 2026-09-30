package api

import (
	"net/url"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/requestid"
)

const internalPrefix = "/_internal"

func isInternalPath(path string) bool {
	if len(path) >= len(internalPrefix) &&
		strings.EqualFold(path[:len(internalPrefix)], internalPrefix) {
		return true
	}
	if decoded, err := url.PathUnescape(path); err == nil && decoded != path {
		return len(decoded) >= len(internalPrefix) &&
			strings.EqualFold(decoded[:len(internalPrefix)], internalPrefix)
	}
	return false
}

func internalGuard() fiber.Handler {
	return func(c fiber.Ctx) error {
		if isInternalPath(c.Path()) {
			if c.Get("X-Real-IP") != "" || c.Get("X-Forwarded-For") != "" {
				return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
					"errors":  []string{"Not Found"},
					"status":  0,
					"request": requestid.FromContext(c),
				})
			}
		}
		return c.Next()
	}
}
