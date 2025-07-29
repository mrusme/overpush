package apprise

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"time"

	"github.com/mrusme/overpush/config"
	"github.com/mrusme/overpush/helpers"
	"github.com/mrusme/overpush/models/message"
	"github.com/mrusme/overpush/models/target"
	"go.uber.org/zap"
)

type Apprise struct {
	cfg       *config.Config
	log       *zap.Logger
	targetCfg target.Target
}

func New(
	cfg *config.Config,
	log *zap.Logger,
	targetCfg target.Target,
) (*Apprise, error) {
	t := new(Apprise)

	t.cfg = cfg
	t.log = log
	t.targetCfg = targetCfg

	return t, nil
}

func (t *Apprise) Load() error {
	t.log.Info("Load target: Apprise")
	return nil
}

func (t *Apprise) Run() error {
	t.log.Info("Run target: Apprise")
	return nil
}

func (t *Apprise) Execute(
	m message.Message,
	appArgs map[string]interface{},
) error {
	var connection string = ""

	if val, ok := t.targetCfg.Args["connection"]; ok {
		connection, ok = helpers.GetFieldValue(
			val.(string),
			appArgs,
		)
		if !ok {
			return errors.New("Could not parse connection argument")
		}
	} else {
		return errors.New("Could not get connection string")
	}

	var prefix string = ""
	if val, ok := t.targetCfg.Args["prefixDestination"]; ok {
		if casted, ok := val.(bool); ok {
			if casted == true {
				prefix = appArgs["destination"].(string) + " "
			}
		} else if casted, ok := val.(string); ok {
			if casted == "true" {
				prefix = appArgs["destination"].(string) + " "
			}
		}
	}

	ctx, _ := context.WithTimeout(context.Background(), 10*time.Second)
	cmd := exec.CommandContext(
		ctx,
		"python",
		t.targetCfg.Args["apprise"].(string),
		"-vv",
		"-t", m.Title,
		"-b", (prefix + m.Message),
		connection,
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return err
	}

	if err := cmd.Wait(); err != nil {
		return err
	}

	return nil
}

func (t *Apprise) Shutdown() error {
	t.log.Info("Shutdown target: Apprise")
	return nil
}
