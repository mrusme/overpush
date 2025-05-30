package user

import (
	"github.com/mrusme/overpush/config"
	"github.com/mrusme/overpush/database"
	"github.com/mrusme/overpush/models/application"
	"github.com/mrusme/overpush/models/user"
)

type Repository struct {
	cfg *config.Config
	db  *database.Database
}

func New(cfg *config.Config, db *database.Database) (*Repository, error) {
	repo := new(Repository)
	repo.cfg = cfg
	repo.db = db

	return repo, nil
}

func (repo *Repository) GetApplication(
	userKey string,
	token string,
) (application.Application, error) {
	if repo.cfg.Database.Enable == true {
		return repo.db.GetApplication(userKey, token)
	} else {
		return repo.cfg.GetApplication(userKey, token)
	}
}

func (repo *Repository) GetUserFromToken(token string) (user.User, error) {
	if repo.cfg.Database.Enable == true {
		return repo.db.GetUserFromToken(token)
	} else {
		return repo.cfg.GetUserFromToken(token)
	}
}
