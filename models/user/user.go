package user

import (
	"github.com/mrusme/overpush/models/application"
)

type User struct {
	Enable       bool
	Key          string
	Applications []application.Application
}
