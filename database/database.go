package database

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mrusme/overpush/config"
	"github.com/mrusme/overpush/models/application"
	"github.com/mrusme/overpush/models/target"
	"github.com/mrusme/overpush/models/user"
	pgxUUID "github.com/vgarvardt/pgx-google-uuid/v5"
	"go.uber.org/zap"
)

type Database struct {
	cfg *config.Config
	log *zap.Logger

	poolcfg *pgxpool.Config
	pool    *pgxpool.Pool
}

var (
	APPLICATION_FIELDS = "enable,token,name,icon_path,format,custom_format,encryption_type,encryption_recipients,encrypt_title,encrypt_message,encrypt_attachment,target_id as target,target_args"
	TARGET_FIELDS      = "id,enable,type,args"
)

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
			db.log.Debug("Registering UUID Types")
			pgxUUID.Register(c.TypeMap())
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

func (db *Database) GetApplication(
	userKey string,
	token string,
) (application.Application, error) {
	if db.cfg.Database.Enable == false {
		return application.Application{}, nil
	}

	ctx, _ := context.WithTimeout(context.Background(), 5*time.Second)
	rows, err := db.pool.Query(ctx,
		"SELECT "+APPLICATION_FIELDS+" FROM applications WHERE token = $1",
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
	if len(applications) == 0 {
		return application.Application{}, errors.New("Application not found")
	}

	return applications[0], nil
}

func (db *Database) GetApplicationsForUser(
	userID string,
) ([]application.Application, error) {
	if db.cfg.Database.Enable == false {
		return []application.Application{}, nil
	}

	ctx, _ := context.WithTimeout(context.Background(), 5*time.Second)
	rows, err := db.pool.Query(ctx,
		"SELECT "+APPLICATION_FIELDS+" FROM applications WHERE user_id = $1",
		userID,
	)
	if err != nil {
		return []application.Application{}, err
	}

	applications, err := pgx.CollectRows[application.Application](
		rows,
		pgx.RowToStructByName[application.Application],
	)
	if err != nil {
		return []application.Application{}, err
	}

	return applications, nil
}

func (db *Database) GetUserFromToken(token string) (user.User, error) {
	if db.cfg.Database.Enable == false {
		return user.User{}, nil
	}

	var userID string
	var enable bool
	var key string

	ctx, _ := context.WithTimeout(context.Background(), 5*time.Second)
	if err := db.pool.QueryRow(ctx,
		"SELECT users.id,users.key,users.enable FROM applications JOIN users ON applications.user_id = users.id WHERE applications.token = $1",
		token,
	).Scan(&userID, &key, &enable); err != nil {
		return user.User{}, err
	}

	applications, err := db.GetApplicationsForUser(userID)
	if err != nil {
		return user.User{}, err
	}

	user := user.User{
		Enable:       enable,
		Key:          key,
		Applications: applications,
	}

	return user, nil
}

func (db *Database) GetTargets() ([]target.Target, error) {
	if db.cfg.Database.Enable == false {
		return []target.Target{}, nil
	}

	ctx, _ := context.WithTimeout(context.Background(), 5*time.Second)
	rows, err := db.pool.Query(ctx,
		"SELECT "+TARGET_FIELDS+" FROM targets",
	)
	if err != nil {
		return []target.Target{}, err
	}

	targets, err := pgx.CollectRows[target.Target](
		rows,
		pgx.RowToStructByName[target.Target],
	)
	if err != nil {
		return []target.Target{}, err
	}

	return targets, nil
}

func (db *Database) GetTargetByID(targetID string) (target.Target, error) {
	if db.cfg.Database.Enable == false {
		return target.Target{}, nil
	}

	var enable bool
	var targetType string
	var targetArgs map[string]interface{}

	ctx, _ := context.WithTimeout(context.Background(), 5*time.Second)
	if err := db.pool.QueryRow(ctx,
		"SELECT "+TARGET_FIELDS+" FROM targets WHERE id = $1",
		targetID,
	).Scan(&targetID, &enable, &targetType, &targetArgs); err != nil {
		return target.Target{}, err
	}

	target := target.Target{
		Enable: enable,
		ID:     targetID,
		Type:   targetType,
		Args:   targetArgs,
	}

	return target, nil
}

func (db *Database) IncrementStat(
	userKey string,
	token string,
	stat string,
) error {
	ctx, _ := context.WithTimeout(context.Background(), 5*time.Second)
	_, err := db.pool.Exec(ctx,
		"UPDATE applications SET stat_"+stat+" = stat_"+stat+" + 1 WHERE token = $1",
		token)
	return err
}

func (db *Database) SaveInput(
	userKey string,
	token string,
	input string,
) error {
	ctx, _ := context.WithTimeout(context.Background(), 5*time.Second)
	_, err := db.pool.Exec(ctx,
		"UPDATE applications SET stat_received = stat_received + 1, latest_input = CASE WHEN store_latest_input THEN $1 ELSE '' END WHERE token = $2",
		input,
		token)
	return err
}
