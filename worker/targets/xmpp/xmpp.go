package xmpp

import (
	"crypto/tls"
	"strconv"
	"strings"

	"github.com/mrusme/overpush/config"
	"github.com/mrusme/overpush/models/message"
	goxmpp "github.com/xmppo/go-xmpp"
	"go.uber.org/zap"
)

type XMPP struct {
	cfg *config.Config
	log *zap.Logger
}

func New(
	cfg *config.Config,
	log *zap.Logger,
) (*XMPP, error) {
	t := new(XMPP)

	t.cfg = cfg
	t.log = log

	return t, nil
}

func (t *XMPP) Load() error {
	t.log.Info("Load target: XMPP")
	return nil
}

func (t *XMPP) Run() error {
	t.log.Info("Run target: XMPP")
	return nil
}

func (t *XMPP) Execute(
	m message.Message,
	args map[string]interface{},
	appArgs map[string]interface{},
) error {
	var jabber *goxmpp.Client

	xmppServer := args["server"].(string)
	xmppTLS, err := strconv.ParseBool(args["tls"].(string))
	if err != nil {
		xmppTLS = true
	}
	xmppUsername := args["username"].(string)
	xmppPassword := args["password"].(string)
	destinationUsername := appArgs["destination"].(string)

	goxmpp.DefaultConfig = &tls.Config{
		ServerName:         strings.Split(xmppServer, ":")[0],
		InsecureSkipVerify: false,
	}

	jabberOpts := goxmpp.Options{
		Host:          xmppServer,
		User:          xmppUsername,
		Password:      xmppPassword,
		NoTLS:         !xmppTLS,
		Debug:         false,
		Session:       true,
		Status:        "xa",
		StatusMessage: "Pushing over ...",
	}

	jabber, err = jabberOpts.NewClient()
	if err != nil {
		t.log.Error("XMPP failed to connect",
			zap.Error(err))
		return err
	}
	defer jabber.Close()

	_, err = jabber.Send(goxmpp.Chat{
		Remote: destinationUsername,
		Type:   "chat",
		Text:   m.ToString(),
	})
	if err != nil {
		t.log.Error("XMPP failed to send",
			zap.Error(err))
		return err
	}

	return nil
}

func (t *XMPP) Shutdown() error {
	t.log.Info("Shutdown target: XMPP")
	return nil
}
