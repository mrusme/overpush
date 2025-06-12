package targets

import (
	"errors"

	"github.com/mrusme/overpush/config"
	"github.com/mrusme/overpush/helpers"
	"github.com/mrusme/overpush/models/message"
	"github.com/mrusme/overpush/models/target"
	"github.com/mrusme/overpush/worker/targets/apprise"
	"github.com/mrusme/overpush/worker/targets/xmpp"
	"go.uber.org/zap"
)

type ITarget interface {
	Load() error
	Run() error
	Execute(
		m message.Message,
		appArgs map[string]interface{},
	) error
	Shutdown() error
}

type (
	ITargets map[string]ITarget
)

type Targets struct {
	cfg        *config.Config
	log        *zap.Logger
	targetCfgs []target.Target
	targets    ITargets
}

func NewTarget(
	cfg *config.Config,
	log *zap.Logger,
	targetCfg target.Target,
) (ITarget, error) {
	var t ITarget
	var err error

	switch targetCfg.Type {
	case "xmpp":
		t, err = xmpp.New(cfg, log, targetCfg)
	case "apprise":
		t, err = apprise.New(cfg, log, targetCfg)
	default:
		return nil, errors.New("No such target type")
	}
	if err != nil {
		return nil, err
	}

	return t, nil
}

func New(
	cfg *config.Config,
	log *zap.Logger,
	targetCfgs []target.Target,
) (*Targets, error) {
	var err error

	ts := new(Targets)

	ts.cfg = cfg
	ts.log = log
	ts.targetCfgs = targetCfgs
	ts.targets = make(ITargets)

	for _, tcfg := range ts.targetCfgs {
		if ts.targets[tcfg.ID], err = NewTarget(cfg, log, tcfg); err != nil {
			return nil, err
		}
	}

	return ts, nil
}

func (ts *Targets) LoadAll() error {
	for _, tcfg := range ts.targetCfgs {
		if err := ts.targets[tcfg.ID].Load(); err != nil {
			return err
		}
	}

	return nil
}

func (ts *Targets) RunAll() error {
	var running []string

	for _, tcfg := range ts.targetCfgs {
		if err := ts.targets[tcfg.ID].Run(); err != nil {
			for _, tnamerunning := range running {
				ts.targets[tnamerunning].Shutdown()
			}
			return err
		}
		running = append(running, tcfg.ID)
	}

	return nil
}

func (ts *Targets) Execute(
	id string,
	m message.Message,
	appArgs map[string]interface{},
) error {
	return ts.targets[id].Execute(m, appArgs)
}

func (ts *Targets) ExecuteAll(
	m message.Message,
	appArgs map[string]interface{},
) (bool, map[string]error) {
	var errs map[string]error = make(map[string]error)
	var ok bool = true

	for _, tcfg := range ts.targetCfgs {
		if err := ts.targets[tcfg.ID].Execute(m, appArgs); err != nil {
			errs[tcfg.ID] = err
			ok = false
		}
	}

	return ok, errs
}

func (ts *Targets) ShutdownAll() (bool, helpers.Errors) {
	var errs helpers.Errors = make(helpers.Errors)
	var ok bool = true

	for _, tcfg := range ts.targetCfgs {
		if err := ts.targets[tcfg.ID].Shutdown(); err != nil {
			errs[tcfg.ID] = err
			ok = false
		}
	}

	return ok, errs
}
