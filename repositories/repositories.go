package repositories

import (
	"github.com/mrusme/overpush/config"
	"github.com/mrusme/overpush/database"
	"github.com/mrusme/overpush/repositories/application"
	"github.com/mrusme/overpush/repositories/target"
	"github.com/mrusme/overpush/repositories/user"
)

type Repositories struct {
	cfg         *config.Config
	db          *database.Database
	User        *user.Repository
	Application *application.Repository
	Target      *target.Repository
}

func New(
	cfg *config.Config,
	db *database.Database,
) (*Repositories, error) {
	var repos *Repositories = new(Repositories)
	var err error

	repos.cfg = cfg
	repos.db = db

	var userRepo *user.Repository
	if userRepo, err = user.New(cfg, db); err != nil {
		return nil, err
	}

	var appRepo *application.Repository
	if appRepo, err = application.New(cfg, db); err != nil {
		return nil, err
	}

	var targetRepo *target.Repository
	if targetRepo, err = target.New(cfg, db); err != nil {
		return nil, err
	}

	repos.User = userRepo
	repos.Application = appRepo
	repos.Target = targetRepo

	return repos, nil
}

func (repos *Repositories) Shutdown() error {
	return repos.db.Shutdown()
}
