package xmpp

import (
	"crypto/tls"
	"fmt"
	"strconv"
	"strings"

	"github.com/mrusme/overpush/api/messages"
	"github.com/mrusme/overpush/config"
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
	m messages.Request,
	args map[string]string,
	appArgs map[string]string,
) error {
	var jabber *goxmpp.Client

	xmppServer := args["server"]
	xmppTLS, err := strconv.ParseBool(args["tls"])
	if err != nil {
		xmppTLS = true
	}
	xmppUsername := args["username"]
	xmppPassword := args["password"]
	destinationUsername := args["destination"]

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
		fmt.Println(err)
		return err
	}
	defer jabber.Close()

	_, err = jabber.Send(goxmpp.Chat{
		Remote: destinationUsername,
		Type:   "chat",
		Text:   m.ToString(),
	})
	if err != nil {
		fmt.Println(err)
		return err
	}

	return nil
}

func (t *XMPP) Shutdown() error {
	t.log.Info("Shutdown target: XMPP")
	return nil
}

