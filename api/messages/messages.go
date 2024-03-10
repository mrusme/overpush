package messages

import (
	"encoding/json"
	"fmt"

	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/requestid"
	"github.com/hibiken/asynq"
)

type Messages struct {
}

type Request struct {
	Token   string `json:"token",validate:"required,printascii"`
	User    string `json:"user",validate:"required,printascii"`
	Message string `json:"message",validate:"required"`

	Attachment       string `json:"attachment",validate:""`
	AttachmentBase64 string `json:"attachment",validate:"base64"`
	AttachmentType   string `json:"attachment_type",validate:""`
	Device           string `json:"device",validate:""`
	HTML             int    `json:"html",validate:"min=0,max=1"`
	Priority         int    `json:"priority",validate:"min=-2,max=2"`
	Timestamp        int    `json:"timestamp",validate:""`
	Title            string `json:"title",validate:""`
	TTL              int    `json:"ttl",validate:""`
	URL              string `json:"url",validate:"http_url"`
	URLTitle         string `json:"url_title",validate:""`
}

func New(app *fiber.App, ac *asynq.Client) {
	validate := validator.New(validator.WithRequiredStructEnabled())

	app.Post("/1/messages.json", func(c fiber.Ctx) error {
		req := new(Request)

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

		fmt.Printf("Req:\n%v\n\n", req)

		payload, err := json.Marshal(req)
		if err != nil {
			return c.JSON(fiber.Map{
				"error":   err.Error(),
				"status":  0,
				"request": requestid.FromContext(c),
			})
		}

		info, err := ac.Enqueue(asynq.NewTask("message", payload))
		if err != nil {
			return c.JSON(fiber.Map{
				"error":   err.Error(),
				"status":  0,
				"request": requestid.FromContext(c),
			})
		}

		fmt.Printf("Enqueued:\n%v\n\n", info)

		return c.JSON(fiber.Map{
			"status":  1,
			"request": requestid.FromContext(c),
		})
	})
}
