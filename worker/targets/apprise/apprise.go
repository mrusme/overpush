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
	"go.uber.org/zap"
)

type Apprise struct {
	cfg *config.Config
	log *zap.Logger
}

func New(
	cfg *config.Config,
	log *zap.Logger,
) (*Apprise, error) {
	t := new(Apprise)

	t.cfg = cfg
	t.log = log

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
	args map[string]string,
	appArgs map[string]string,
) error {
	connection, ok := helpers.GetFieldValue(args["connection"], appArgs)
	if !ok {
		return errors.New("Could not parse connection argument")
	}

	ctx, _ := context.WithTimeout(context.Background(), 10*time.Second)
	cmd := exec.CommandContext(
		ctx,
		"python",
		args["apprise"],
		"-vv",
		"-t", m.Title,
		"-b", m.Message,
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
