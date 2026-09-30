package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hibiken/asynq"
	"github.com/mrusme/overpush/config"
	"github.com/mrusme/overpush/database"
	"github.com/mrusme/overpush/models/application"
	"github.com/mrusme/overpush/models/message"
	"github.com/mrusme/overpush/models/target"
	"github.com/mrusme/overpush/models/user"
	"github.com/mrusme/overpush/repositories"
	"github.com/mrusme/overpush/worker/targets"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

const (
	wtUserKey = "worker-test-user-key-000"
	wtToken   = "worker-test-token-000000"
)

func wtRecorder(t *testing.T, exitCode int) (string, string) {
	t.Helper()
	if _, err := exec.LookPath("python"); err != nil {
		t.Skip("python not available")
	}
	dir := t.TempDir()
	outPath := filepath.Join(dir, "argv.txt")
	script := fmt.Sprintf(
		"import sys\n"+
			"open(%q, \"w\").write(\"\\n\".join(sys.argv[1:]))\n"+
			"sys.exit(%d)\n",
		outPath, exitCode)
	scriptPath := filepath.Join(dir, "recorder.py")
	if err := os.WriteFile(scriptPath, []byte(script), 0o644); err != nil {
		t.Fatal(err)
	}
	return scriptPath, outPath
}

func wtConfig(recorderPath string, destination interface{}) *config.Config {
	cfg := &config.Config{}
	cfg.Worker.Enable = true
	cfg.Users = []user.User{
		{
			Enable: true,
			Key:    wtUserKey,
			Applications: []application.Application{
				{
					Enable: true,
					Token:  wtToken,
					Name:   "Worker Test",
					Format: "pushover",
					Target: "apprise-test",
					TargetArgs: map[string]interface{}{
						"destination": destination,
					},
				},
			},
		},
	}
	cfg.Targets = []target.Target{
		{
			Enable: true,
			ID:     "apprise-test",
			Type:   "apprise",
			Args: map[string]interface{}{
				"apprise":             recorderPath,
				"destination_pattern": `^(-?[0-9]{1,20}|@[A-Za-z0-9_]{5,32})$`,
				"connection":          `proto://SYNTHETIC-WORKER-SECRET/{{ arg "destination" }}`,
			},
		},
	}
	return cfg
}

func newTestWorker(
	t *testing.T,
	cfg *config.Config,
) (*Worker, *observer.ObservedLogs) {
	t.Helper()

	core, logs := observer.New(zap.DebugLevel)
	log := zap.New(core)

	db, err := database.New(cfg, log)
	if err != nil {
		t.Fatal(err)
	}
	repos, err := repositories.New(cfg, db)
	if err != nil {
		t.Fatal(err)
	}

	targetCfgs, err := repos.Target.GetTargets()
	if err != nil {
		t.Fatal(err)
	}
	ts, err := targets.New(cfg, log, targetCfgs)
	if err != nil {
		t.Fatal(err)
	}
	if err := ts.LoadAll(); err != nil {
		t.Fatal(err)
	}
	if err := ts.RunAll(); err != nil {
		t.Fatal(err)
	}

	wrk, err := New(cfg, log)
	if err != nil {
		t.Fatal(err)
	}
	wrk.repos = repos
	wrk.ts = ts

	return wrk, logs
}

func wtTask(t *testing.T, msg message.Message) *asynq.Task {
	t.Helper()
	payload, err := json.Marshal(&msg)
	if err != nil {
		t.Fatal(err)
	}
	return asynq.NewTask("message", payload)
}

func TestHandleMessageDeliversAndLogsNoSecrets(t *testing.T) {
	recorderPath, outPath := wtRecorder(t, 0)
	wrk, logs := newTestWorker(t, wtConfig(recorderPath, "4242"))

	msg := message.Message{
		Token:   wtToken,
		User:    wtUserKey,
		Title:   "SYNTHETIC WORKER TITLE",
		Message: "SYNTHETIC WORKER BODY",
	}
	if err := wrk.HandleMessage(context.Background(), wtTask(t, msg)); err != nil {
		t.Fatal(err)
	}

	argvRaw, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatal(err)
	}
	argv := strings.Split(string(argvRaw), "\n")
	if argv[len(argv)-1] != "proto://SYNTHETIC-WORKER-SECRET/4242" {
		t.Fatalf("connection argv = %q", argv[len(argv)-1])
	}

	for _, entry := range logs.All() {
		line := entry.Message + fmt.Sprint(entry.ContextMap())
		for _, secret := range []string{
			wtToken, wtUserKey, "SYNTHETIC WORKER TITLE",
			"SYNTHETIC WORKER BODY", "SYNTHETIC-WORKER-SECRET", "4242",
		} {
			if strings.Contains(line, secret) {
				t.Fatalf("log line %q contains %q", line, secret)
			}
		}
	}
}

func TestHandleMessageRejectsInjectedDestinationPermanently(t *testing.T) {
	recorderPath, outPath := wtRecorder(t, 0)
	wrk, _ := newTestWorker(t,
		wtConfig(recorderPath, "4242 json://127.0.0.1/"))

	msg := message.Message{
		Token:   wtToken,
		User:    wtUserKey,
		Message: "body",
	}
	err := wrk.HandleMessage(context.Background(), wtTask(t, msg))
	if err == nil || !errors.Is(err, asynq.SkipRetry) {
		t.Fatalf("expected permanent error, got %v", err)
	}
	if _, statErr := os.Stat(outPath); statErr == nil {
		t.Fatal("recorder was invoked for an injected destination")
	}
}

func TestHandleMessageUnwrapsPerTargetArgs(t *testing.T) {
	recorderPath, outPath := wtRecorder(t, 0)
	cfg := wtConfig(recorderPath, "ignored")
	cfg.Users[0].Applications[0].TargetArgs = map[string]interface{}{
		"apprise-test": map[string]interface{}{
			"destination": "9999",
		},
	}
	wrk, _ := newTestWorker(t, cfg)

	msg := message.Message{
		Token:   wtToken,
		User:    wtUserKey,
		Message: "body",
	}
	if err := wrk.HandleMessage(context.Background(), wtTask(t, msg)); err != nil {
		t.Fatal(err)
	}
	argvRaw, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(argvRaw), "proto://SYNTHETIC-WORKER-SECRET/9999") {
		t.Fatalf("argv = %q", argvRaw)
	}
}

func TestHandleMessageDisabledApplication(t *testing.T) {
	recorderPath, outPath := wtRecorder(t, 0)
	cfg := wtConfig(recorderPath, "4242")
	cfg.Users[0].Applications[0].Enable = false
	wrk, logs := newTestWorker(t, cfg)

	msg := message.Message{
		Token:   wtToken,
		User:    wtUserKey,
		Message: "body",
	}
	if err := wrk.HandleMessage(context.Background(), wtTask(t, msg)); err != nil {
		t.Fatal(err)
	}
	if _, statErr := os.Stat(outPath); statErr == nil {
		t.Fatal("recorder was invoked for a disabled application")
	}
	for _, entry := range logs.All() {
		line := entry.Message + fmt.Sprint(entry.ContextMap())
		if strings.Contains(line, wtToken) {
			t.Fatalf("log line %q contains the token", line)
		}
	}
}
