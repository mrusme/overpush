package target

import (
	"github.com/mrusme/overpush/config"
	"github.com/mrusme/overpush/database"
	"github.com/mrusme/overpush/models/target"
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

func (repo *Repository) GetTargets() ([]target.Target, error) {
	if repo.cfg.Database.Enable == true {
		return repo.db.GetTargets()
	} else {
		return repo.cfg.GetTargets()
	}
}

func (repo *Repository) GetTargetByID(targetID string) (target.Target, error) {
	if repo.cfg.Database.Enable == true {
		return repo.db.GetTargetByID(targetID)
	} else {
		return repo.cfg.GetTargetByID(targetID)
	}
}
