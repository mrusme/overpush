package application

import (
	"github.com/mrusme/overpush/config"
	"github.com/mrusme/overpush/database"
	"github.com/mrusme/overpush/models/application"
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

func (repo *Repository) IncrementStat(
	userKey string,
	token string,
	stat string,
) (error) {
	if repo.cfg.Database.Enable == true {
		return repo.db.IncrementStat(userKey, token, stat)
	} else {
		// We don't count stats for config based setups
		return nil
	}
}

func (repo *Repository) SaveInput(
	userKey string,
	token string,
	input string,
) (error) {
	if repo.cfg.Database.Enable == true {
		return repo.db.SaveInput(userKey, token, input)
	} else {
		// We don't save inputs for config based setups
		return nil
	}
}
