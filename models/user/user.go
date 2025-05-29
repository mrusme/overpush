package user

import (
	"github.com/mrusme/overpush/models/application"
)

type User struct {
	Key          string
	Applications []application.Application
}
