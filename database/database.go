package database

import (
	"context"
	"time"

	pgxuuid "github.com/jackc/pgx-gofrs-uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mrusme/overpush/config"
	"github.com/mrusme/overpush/models/application"
	"github.com/mrusme/overpush/models/target"
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

func (db *Database) Query(q string, args ...any) (pgx.Rows, error) {
	ctx, _ := context.WithTimeout(context.Background(), 5*time.Second)
	rows, err := db.pool.Query(ctx, q, args)
	if err != nil {
		return nil, err
	}

	return rows, nil
}

func (db *Database) QueryOne(q string, args ...any) pgx.Row {
	ctx, _ := context.WithTimeout(context.Background(), 5*time.Second)
	row := db.pool.QueryRow(ctx, q, args)

	return row
}

func (db *Database) GetApplication(
	userKey string,
	token string,
) (application.Application, error) {
	if db.cfg.Database.Enable == false {
		return application.Application{}, nil
	}

	rows, err := db.Query(
		"SELECT * FROM applications WHERE user_key = $1 AND token = $2",
		userKey,
		token,
	)
	if err != nil {
		return application.Application{}, err
	}

	applications, err := pgx.CollectRows[application.Application](
		rows,
		pgx.RowToStructByName[application.Application],
	)
	if err != nil {
		return application.Application{}, err
	}

	return applications[0], nil
}

func (db *Database) GetUserKeyFromToken(token string) (string, error) {
	if db.cfg.Database.Enable == false {
		return "", nil
	}

	row := db.QueryOne(
		"SELECT user_key FROM applications WHERE token = $1",
		token,
	)

	var userKey string
	if err := row.Scan(&userKey); err != nil {
		return "", err
	}

	return userKey, nil
}

func (db *Database) GetTargetByID(targetID string) (target.Target, error) {
	if db.cfg.Database.Enable == false {
		return target.Target{}, nil
	}

	rows, err := db.Query(
		"SELECT * FROM targets WHERE id = $1",
		targetID,
	)
	if err != nil {
		return target.Target{}, err
	}

	targets, err := pgx.CollectRows[target.Target](
		rows,
		pgx.RowToStructByName[target.Target],
	)
	if err != nil {
		return target.Target{}, err
	}

	return targets[0], nil
}
