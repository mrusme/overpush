package database

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	pgxuuid "github.com/jackc/pgx-gofrs-uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mrusme/overpush/config"
	"go.uber.org/zap"
)

type Database struct {
	cfg *config.Config
	log *zap.Logger

	poolcfg *pgxpool.Config
	pool    *pgxpool.Pool
}

func New(cfg *config.Config, log *zap.Logger) (*Database, error) {
	var err error

	db := new(Database)
	db.cfg = cfg
	db.log = log

	if db.cfg.Database.Enable == true {
		if db.poolcfg, err = pgxpool.ParseConfig(db.cfg.Database.Connection); err != nil {
			return nil, err
		}

		db.poolcfg.AfterConnect = func(ctx context.Context, c *pgx.Conn) error {
			pgxuuid.Register(c.TypeMap())
			return nil
		}

		if db.pool, err = pgxpool.NewWithConfig(
			context.Background(),
			db.poolcfg,
		); err != nil {
			return nil, err
		}

		db.log.Info("Database initialized")

		var greeting string
		ctx, _ := context.WithTimeout(context.Background(), 5*time.Second)
		err = db.pool.QueryRow(ctx, "select 'Hello, world!'").Scan(&greeting)
		if err != nil {
			db.Shutdown()
			return db, err
		}
		db.log.Info("Database connected",
			zap.String("greeting", greeting))
	} else {
		db.log.Debug("Database not enabled",
			zap.Bool("Database.Enable", db.cfg.Database.Enable))
	}

	return db, nil
}

func (db *Database) Shutdown() error {
	if db.cfg.Database.Enable == true {
		db.pool.Close()
	}
	return nil
}
