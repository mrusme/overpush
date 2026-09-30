package apprise

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"time"

	"github.com/hibiken/asynq"
	"github.com/mrusme/overpush/config"
	"github.com/mrusme/overpush/helpers"
	"github.com/mrusme/overpush/models/message"
	"github.com/mrusme/overpush/models/target"
	"go.uber.org/zap"
)

const maxOutputBytes = 64 * 1024

var defaultDestinationPattern = regexp.MustCompile(
	`^[^\s,?&/\\[:cntrl:]]+$`)

type boundedBuffer struct {
	buf bytes.Buffer
}

func (bb *boundedBuffer) Write(p []byte) (int, error) {
	remaining := maxOutputBytes - bb.buf.Len()
	if remaining > 0 {
		if len(p) > remaining {
			bb.buf.Write(p[:remaining])
		} else {
			bb.buf.Write(p)
		}
	}
	return len(p), nil
}

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

func (t *Apprise) validDestination(destination string) (bool, error) {
	pattern := defaultDestinationPattern
	if val, ok := t.targetCfg.Args["destination_pattern"].(string); ok &&
		val != "" {
		compiled, err := regexp.Compile(val)
		if err != nil {
			return false, fmt.Errorf(
				"target has an invalid destination_pattern: %w",
				asynq.SkipRetry)
		}
		pattern = compiled
	}

	return destination != "" && pattern.MatchString(destination), nil
}

func (t *Apprise) Execute(
	m message.Message,
	appArgs map[string]interface{},
) error {
	destination, _ := appArgs["destination"].(string)

	ok, err := t.validDestination(destination)
	if err != nil {
		t.log.Error("Apprise destination_pattern does not compile",
			zap.String("Target.ID", t.targetCfg.ID))
		return err
	}
	if !ok {
		t.log.Error("Apprise destination rejected",
			zap.String("Target.ID", t.targetCfg.ID))
		return fmt.Errorf(
			"destination does not match the target's destination pattern: %w",
			asynq.SkipRetry)
	}

	var connection string = ""

	if val, ok := t.targetCfg.Args["connection"].(string); ok {
		connection, ok = helpers.GetFieldValue(
			val,
			appArgs,
		)
		if !ok {
			return fmt.Errorf("could not parse connection argument: %w",
				asynq.SkipRetry)
		}
	} else {
		return fmt.Errorf("could not get connection string: %w",
			asynq.SkipRetry)
	}

	var prefix string = ""
	if val, ok := t.targetCfg.Args["prefixDestination"]; ok {
		if casted, ok := val.(bool); ok {
			if casted == true {
				prefix = destination + " "
			}
		} else if casted, ok := val.(string); ok {
			if casted == "true" {
				prefix = destination + " "
			}
		}
	}

	appriseBin, ok2 := t.targetCfg.Args["apprise"].(string)
	if !ok2 || appriseBin == "" {
		return fmt.Errorf("could not get apprise binary path: %w",
			asynq.SkipRetry)
	}

	t.log.Debug("Apprise executing",
		zap.String("Target.ID", t.targetCfg.ID),
		zap.String("appriseBin", appriseBin),
	)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(
		ctx,
		"python",
		appriseBin,
		"-vv",
		"-t", (prefix + m.Title),
		"-b", (prefix + m.Message),
		connection,
	)
	output := new(boundedBuffer)
	cmd.Stdout = output
	cmd.Stderr = output

	if err := cmd.Start(); err != nil {
		return err
	}

	if err := cmd.Wait(); err != nil {
		redacted := bytes.ReplaceAll(
			output.buf.Bytes(),
			[]byte(connection),
			[]byte("<connection>"),
		)
		t.log.Error("Apprise failed",
			zap.String("Target.ID", t.targetCfg.ID),
			zap.ByteString("output", redacted),
		)
		return err
	}

	return nil
}

func (t *Apprise) Shutdown() error {
	t.log.Info("Shutdown target: Apprise")
	return nil
}
