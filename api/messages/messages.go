package messages

import (
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/requestid"
)

type Messages struct {
}

func New(app *fiber.App) {
	app.Post("/1/messages.json", func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"status":  1,
			"request": requestid.FromContext(c),
		})
	})
}
