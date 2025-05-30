package api

import (
	"context"
	"encoding/json"
	"strconv"

	"github.com/Jeffail/gabs/v2"
	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/requestid"
	"github.com/hibiken/asynq"
	"github.com/markusmobius/go-dateparser"
	"github.com/mrusme/overpush/api/messages"
	"github.com/mrusme/overpush/models/application"
	"github.com/mrusme/overpush/worker"
	"go.uber.org/zap"
)

func handler(api *API) func(c fiber.Ctx) error {
	return func(c fiber.Ctx) error {
		var user string
		var token string
		var msg *messages.Request
		var appFormat string
		var application application.Application
		var err error

		validate := validator.New(validator.WithRequiredStructEnabled())

		bound := c.Bind()

		api.log.Debug("Received request, processing ...")

		if c.Route().Path == "/1/messages.json" {
			appFormat = "pushover"
		} else {
			token = c.Params("token")
			user, err = api.repos.User.GetUserKeyFromToken(token)
			if err != nil {
				return c.Status(fiber.ErrBadRequest.Code).JSON(fiber.Map{
					"error":   err.Error(),
					"status":  0,
					"request": requestid.FromContext(c),
				})
			}

			api.log.Debug("Retrieving application ...")
			application, err = api.repos.User.GetApplication(user, token)
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
			api.log.Debug("Calling worker directly with request",
				zap.ByteString("payload", payload))

			wrk, err := worker.New(api.cfg, api.log, api.repos)
			if err != nil {
				api.log.Error("Error calling worker directly", zap.Error(err))
				return c.Status(fiber.ErrInternalServerError.Code).JSON(fiber.Map{
					"error":   err.Error(),
					"status":  0,
					"request": requestid.FromContext(c),
				})
			}

			wrk.HandleMessage(context.Background(), asynq.NewTask("message", payload))
		}

		return c.JSON(fiber.Map{
			"status":  1,
			"request": requestid.FromContext(c),
		})
	}
}
