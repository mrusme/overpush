package xmpp

import (
	"crypto/tls"
	"strconv"
	"strings"

	"github.com/mrusme/overpush/config"
	"github.com/mrusme/overpush/models/message"
	"github.com/mrusme/overpush/models/target"
	goxmpp "github.com/xmppo/go-xmpp"
	"go.uber.org/zap"
)

type XMPP struct {
	cfg       *config.Config
	log       *zap.Logger
	targetCfg target.Target

	jabberOpts goxmpp.Options
	jabber     *goxmpp.Client
}

func New(
	cfg *config.Config,
	log *zap.Logger,
	targetCfg target.Target,
) (*XMPP, error) {
	t := new(XMPP)

	t.cfg = cfg
	t.log = log
	t.targetCfg = targetCfg

	return t, nil
}

func (t *XMPP) Load() error {
	t.log.Info("Load target: XMPP")
	xmppServer := t.targetCfg.Args["server"].(string)
	xmppTLS, err := strconv.ParseBool(t.targetCfg.Args["tls"].(string))
	if err != nil {
		xmppTLS = true
	}
	xmppUsername := t.targetCfg.Args["username"].(string)
	xmppPassword := t.targetCfg.Args["password"].(string)

	goxmpp.DefaultConfig = &tls.Config{
		ServerName:         strings.Split(xmppServer, ":")[0],
		InsecureSkipVerify: false,
	}

	t.jabberOpts = goxmpp.Options{
		Host:                xmppServer,
		User:                xmppUsername,
		Password:            xmppPassword,
		NoTLS:               true,
		StartTLS:            xmppTLS,
		Debug:               false,
		Session:             true,
		Status:              "xa",
		StatusMessage:       "Pushing over ...",
		PeriodicServerPings: true,
	}

	return nil
}

func (t *XMPP) Run() error {
	t.log.Info("Run target: XMPP")

	return t.reconnect()
}

func (t *XMPP) reconnect() error {
	var err error

	if t.jabber != nil {
		t.log.Debug("XMPP close existing client")
		t.jabber.Close()
	}

	t.log.Debug("XMPP connect to server ...",
		zap.String("Host", t.jabberOpts.Host))
	t.jabber, err = t.jabberOpts.NewClient()
	if err != nil {
		t.log.Error("XMPP failed to connect",
			zap.Error(err))
		return err
	}

	return nil
}

func (t *XMPP) Execute(
	m message.Message,
	appArgs map[string]interface{},
) error {
	var err error

	destinationUsername := appArgs["destination"].(string)

	_, err = t.jabber.SendKeepAlive()
	if err != nil {
		t.log.Error("XMPP failed to SendKeepAlive, attempting reconnect ...",
			zap.Error(err))
		if err = t.reconnect(); err != nil {
			return err
		}
	}

	_, err = t.jabber.Send(goxmpp.Chat{
		Remote: destinationUsername,
		Type:   "chat",
		Text:   m.ToString(),
	})
	if err != nil {
		t.log.Error("XMPP failed to send",
			zap.Error(err))
		return err
	}

	t.log.Debug("XMPP successfully sent message",
		zap.String("destinationUsername", destinationUsername))

	return nil
}

func (t *XMPP) Shutdown() error {
	t.log.Info("Shutdown target: XMPP")

	t.jabber.Close()

	return nil
}
