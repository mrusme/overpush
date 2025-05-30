package targets

import (
	"errors"

	"github.com/mrusme/overpush/config"
	"github.com/mrusme/overpush/helpers"
	"github.com/mrusme/overpush/models/message"
	"github.com/mrusme/overpush/worker/targets/apprise"
	"github.com/mrusme/overpush/worker/targets/xmpp"
	"go.uber.org/zap"
)

var TARGETS []string = []string{
	"apprise",
	"xmpp",
}

type Type interface {
	Load() error
	Run() error
	Execute(
		m message.Message,
		args map[string]string,
		appArgs map[string]string,
	) error
	Shutdown() error
}

type Types map[string]Type

type Targets struct {
	cfg     *config.Config
	log     *zap.Logger
	targets Types
}

func NewTarget(
	cfg *config.Config,
	log *zap.Logger,
	name string,
) (Type, error) {
	var t Type
	var err error

	switch name {
	case "xmpp":
		t, err = xmpp.New(cfg, log)
	case "apprise":
		t, err = apprise.New(cfg, log)
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
) (*Targets, error) {
	var err error

	ts := new(Targets)

	ts.cfg = cfg
	ts.log = log
	ts.targets = make(Types)

	for _, tname := range TARGETS {
		if ts.targets[tname], err = NewTarget(cfg, log, tname); err != nil {
			return nil, err
		}
	}

	return ts, nil
}

func (ts *Targets) LoadAll() error {
	for _, tname := range TARGETS {
		if err := ts.targets[tname].Load(); err != nil {
			return err
		}
	}

	return nil
}

func (ts *Targets) RunAll() error {
	var running []string

	for _, tname := range TARGETS {
		if err := ts.targets[tname].Run(); err != nil {
			for _, tnamerunning := range running {
				ts.targets[tnamerunning].Shutdown()
			}
			return err
		}
		running = append(running, tname)
	}

	return nil
}

func (ts *Targets) Execute(
	name string,
	m message.Message,
	args map[string]string,
	appArgs map[string]string,
) error {
	return ts.targets[name].Execute(m, args, appArgs)
}

func (ts *Targets) ExecuteAll(
	m message.Message,
	args map[string]string,
	appArgs map[string]string,
) (bool, map[string]error) {
	var errs map[string]error = make(map[string]error)
	var ok bool = true

	for _, tname := range TARGETS {
		if err := ts.targets[tname].Execute(m, args, appArgs); err != nil {
			errs[tname] = err
			ok = false
		}
	}

	return ok, errs
}

func (ts *Targets) ShutdownAll() (bool, helpers.Errors) {
	var errs helpers.Errors = make(helpers.Errors)
	var ok bool = true

	for _, tname := range TARGETS {
		if err := ts.targets[tname].Shutdown(); err != nil {
			errs[tname] = err
			ok = false
		}
	}

	return ok, errs
}
