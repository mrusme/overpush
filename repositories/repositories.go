package repositories

import (
	"github.com/mrusme/overpush/repositories/target"
	"github.com/mrusme/overpush/repositories/user"
)

type Repositories struct {
	User *user.Repository
	Target *target.Repository
}
