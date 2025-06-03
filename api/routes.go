package api

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/Jeffail/gabs/v2"
	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/requestid"
	"github.com/hibiken/asynq"
	"github.com/markusmobius/go-dateparser"
	"github.com/mrusme/overpush/models/application"
	"github.com/mrusme/overpush/models/message"
	"github.com/mrusme/overpush/models/user"
	"github.com/mrusme/overpush/worker"
	"go.uber.org/zap"
)

func handler(api *API) func(c fiber.Ctx) error {
	return func(c fiber.Ctx) error {
		var viaSubmit bool = false
		var user user.User
		var token string
		var msg *message.Message = new(message.Message)
		var req map[string]interface{}
		var appFormat string
		var application application.Application
		var err error

		validate := validator.New(validator.WithRequiredStructEnabled())

		bound := c.Bind()

		api.log.Debug("Received request, processing ...")

		if c.Route().Path == "/1/messages.json" ||
			c.Route().Path == "/_internal/submit/:token" {
			if c.Route().Path == "/_internal/submit/:token" {
				api.log.Debug("Request received via submit",
					zap.String("IP", c.IP()),
					zap.Strings("IPs", c.IPs()),
				)
				viaSubmit = true
			}

			appFormat = "pushover"
			api.log.Debug("Application is pushover, processing ...")

			if err := bound.Body(msg); err != nil {
				return c.Status(fiber.ErrBadRequest.Code).JSON(fiber.Map{
					"error":   err.Error(),
					"status":  0,
					"request": requestid.FromContext(c),
				})
			}

			if viaSubmit == true {
				token = c.Params("token")
				msg.Token = token
			} else {
				token = msg.Token
			}
		} else {
			appFormat = application.Format
			api.log.Debug("Application is custom, processing ...",
				zap.String("Application.Format", application.Format))

			token = c.Params("token")
		}

		// Validate only the token to make sure it's not foo
		t := reflect.TypeOf(msg).Elem()
		f, _ := t.FieldByName("Token")
		tag, _ := f.Tag.Lookup("validate")
		if err := validate.Var(token, tag); err != nil {
			return c.Status(fiber.ErrBadRequest.Code).JSON(fiber.Map{
				"error":   err.Error(),
				"status":  0,
				"request": requestid.FromContext(c),
			})
		}

		api.log.Debug("Retrieving user from token ...",
			zap.String("token", token))
		user, err = api.repos.User.GetUserFromToken(token)
		if err != nil ||
			(viaSubmit == false && user.Enable == false) {
			api.log.Debug("Could not retrieve user", zap.Error(err))
			return c.Status(fiber.ErrNotFound.Code).JSON(fiber.Map{
				"error":   "No active user with supplied token",
				"status":  0,
				"request": requestid.FromContext(c),
			})
		}

		api.log.Debug("Retrieving application from User.Key and token ...",
			zap.String("User.Key", user.Key),
			zap.String("token", token))
		application, err = api.repos.Application.GetApplication(user.Key, token)
		if err != nil ||
			(viaSubmit == false && application.Enable == false) {
			return c.Status(fiber.ErrNotFound.Code).JSON(fiber.Map{
				"error":   "No active application with supplied token",
				"status":  0,
				"request": requestid.FromContext(c),
			})
		}

		var input strings.Builder
		var pretty []byte

		if appFormat == "pushover" {
			pretty, err = json.MarshalIndent(msg, "", "  ")
		} else {
			req = make(map[string]interface{})

			if err := bound.Body(&req); err != nil {
				api.log.Error("Error parsing", zap.Error(err))
				return c.Status(fiber.ErrBadRequest.Code).JSON(fiber.Map{
					"error":   err.Error(),
					"status":  0,
					"request": requestid.FromContext(c),
				})
			}

			pretty, err = json.MarshalIndent(req, "", "  ")
		}
		if err != nil {
			pretty = c.Body()
		}

		input.WriteString("--- HEADERS --------------------------------------------------------------------\n")
		for k, v := range c.GetReqHeaders() {
			input.WriteString(fmt.Sprintf("%s: %s\n", k, v))
		}
		input.WriteString("\n")

		input.WriteString("--- QUERIES --------------------------------------------------------------------\n")
		for k, v := range c.Queries() {
			input.WriteString(fmt.Sprintf("%s: %s\n", k, v))
		}
		input.WriteString("\n")

		input.WriteString("--- BODY -----------------------------------------------------------------------\n")
		input.WriteString(string(pretty))

		if err = api.repos.Application.SaveInput(
			"No need when DB",
			token,
			input.String(),
		); err != nil {
			api.log.Error("Application input not saved",
				zap.Error(err))
		}

		if appFormat == "pushover" {
			// Nothing yet
		} else {
			locations := make(map[string]*gabs.Container)
			locations["body"] = gabs.Wrap(req)
			var found bool
			var tmp string

			msg.Token = token
			msg.User = user.Key

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

		api.log.Debug("Validating request...")
		if err := validate.Struct(msg); err != nil {
			api.log.Error("Error validating", zap.Error(err))
			return c.Status(fiber.ErrBadRequest.Code).JSON(fiber.Map{
				"error":   err.Error(),
				"status":  0,
				"request": requestid.FromContext(c),
			})
		}

		// Make sure to clear anything that might come from outside on the
		// `internal` struct
		msg.ClearInternal()
		// Set whether message was submitted via /_internal/submit/:token
		msg.SetViaSubmit(viaSubmit)

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

			wrk, err := worker.New(api.cfg, api.log)
			if err != nil {
				api.log.Error("Error calling worker directly", zap.Error(err))
				return c.Status(fiber.ErrInternalServerError.Code).JSON(fiber.Map{
					"error":   err.Error(),
					"status":  0,
					"request": requestid.FromContext(c),
				})
			}

			wrk.Run()

			wrk.HandleMessage(context.Background(), asynq.NewTask("message", payload))
		}

		if err = api.repos.Application.IncrementStat(
			"No need when DB",
			token,
			"processed",
		); err != nil {
			api.log.Error("Application stat not increased",
				zap.String("stat", "processed"),
				zap.Error(err))
		}

		return c.JSON(fiber.Map{
			"status":  1,
			"request": requestid.FromContext(c),
		})
	}
}
