package apprise

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hibiken/asynq"
	"github.com/mrusme/overpush/config"
	"github.com/mrusme/overpush/models/message"
	"github.com/mrusme/overpush/models/target"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

const (
	telegramPattern = `^(-?[0-9]{1,20}|@[A-Za-z0-9_]{5,32})$`
	mastodonPattern = `^@?[A-Za-z0-9_]{1,30}(@[A-Za-z0-9.-]{1,253})?$`
	matrixPattern   = `^[!#@][A-Za-z0-9._=+-]{1,255}:[A-Za-z0-9.-]{1,253}(:[0-9]{1,5})?$`
)

func requirePython(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("python"); err != nil {
		t.Skip("python not available")
	}
}

func writeRecorder(t *testing.T, exitCode int) (string, string) {
	t.Helper()
	dir := t.TempDir()
	outPath := filepath.Join(dir, "argv.txt")
	script := fmt.Sprintf(
		"import sys\n"+
			"open(%q, \"w\").write(\"\\n\".join(sys.argv[1:]))\n"+
			"print(\"recorder output\", \" \".join(sys.argv[1:]))\n"+
			"sys.exit(%d)\n",
		outPath, exitCode)
	scriptPath := filepath.Join(dir, "recorder.py")
	if err := os.WriteFile(scriptPath, []byte(script), 0o644); err != nil {
		t.Fatal(err)
	}
	return scriptPath, outPath
}

func newTestTarget(
	t *testing.T,
	args map[string]interface{},
) (*Apprise, *observer.ObservedLogs) {
	t.Helper()
	core, logs := observer.New(zap.DebugLevel)
	tgt, err := New(&config.Config{}, zap.New(core), target.Target{
		Enable: true,
		ID:     "test-target",
		Type:   "apprise",
		Args:   args,
	})
	if err != nil {
		t.Fatal(err)
	}
	return tgt, logs
}

func TestValidDestinationDefaultPattern(t *testing.T) {
	tgt, _ := newTestTarget(t, map[string]interface{}{})

	valid := []string{"4242", "@somechannel", "someone@example.social",
		"!room=x:example.com", "a.b-c_d:8448"}
	for _, d := range valid {
		ok, err := tgt.validDestination(d)
		if err != nil || !ok {
			t.Fatalf("default pattern rejected %q (err=%v)", d, err)
		}
	}

	invalid := []string{
		"",
		"4242 json://127.0.0.1/",
		"a,b",
		"x?y=z",
		"x&y=z",
		"x/y",
		`x\y`,
		"x\ny",
		"x\ty",
	}
	for _, d := range invalid {
		ok, err := tgt.validDestination(d)
		if err != nil {
			t.Fatalf("unexpected error for %q: %v", d, err)
		}
		if ok {
			t.Fatalf("default pattern accepted %q", d)
		}
	}
}

func TestValidDestinationPerTargetPatterns(t *testing.T) {
	cases := []struct {
		pattern string
		valid   []string
		invalid []string
	}{
		{
			telegramPattern,
			[]string{"4242", "-1001234567890", "@some_channel"},
			[]string{"4242 json://127.0.0.1/", "@ab", "abc", "4242,5353"},
		},
		{
			mastodonPattern,
			[]string{"someone", "@someone", "someone@example.social"},
			[]string{"someone@example.social x", "some?one", "a@b@c"},
		},
		{
			matrixPattern,
			[]string{"!abc123:example.com", "#room:example.com:8448",
				"@person:example.com"},
			[]string{"!abc 123:example.com", "room:example.com",
				"!abc:example.com/extra"},
		},
	}

	for _, tc := range cases {
		tgt, _ := newTestTarget(t, map[string]interface{}{
			"destination_pattern": tc.pattern,
		})
		for _, d := range tc.valid {
			if ok, err := tgt.validDestination(d); err != nil || !ok {
				t.Fatalf("pattern %q rejected %q (err=%v)", tc.pattern, d, err)
			}
		}
		for _, d := range tc.invalid {
			if ok, err := tgt.validDestination(d); err != nil || ok {
				t.Fatalf("pattern %q accepted %q (err=%v)", tc.pattern, d, err)
			}
		}
	}
}

func TestExecuteRejectsInjectedDestination(t *testing.T) {
	requirePython(t)
	scriptPath, outPath := writeRecorder(t, 0)
	tgt, _ := newTestTarget(t, map[string]interface{}{
		"apprise":    scriptPath,
		"connection": `proto://bot:SYNTHETIC-CONNECTION-SECRET@host/{{ arg "destination" }}`,
	})

	for _, d := range []interface{}{
		"4242 json://127.0.0.1/",
		"a,b",
		"x?y=z",
		"x#y{{ arg \"destination\" }}",
		"",
		nil,
		42,
	} {
		args := map[string]interface{}{"destination": d}
		err := tgt.Execute(message.Message{Title: "t", Message: "m"}, args)
		if err == nil {
			t.Fatalf("destination %v: expected error", d)
		}
		if !errors.Is(err, asynq.SkipRetry) {
			t.Fatalf("destination %v: error not permanent: %v", d, err)
		}
		if _, statErr := os.Stat(outPath); statErr == nil {
			t.Fatalf("destination %v: recorder was invoked", d)
		}
	}
}

func TestExecuteInvalidPatternIsPermanent(t *testing.T) {
	tgt, _ := newTestTarget(t, map[string]interface{}{
		"destination_pattern": "([",
	})
	err := tgt.Execute(message.Message{}, map[string]interface{}{
		"destination": "4242",
	})
	if err == nil || !errors.Is(err, asynq.SkipRetry) {
		t.Fatalf("expected permanent error, got %v", err)
	}
}

func TestExecuteMissingConfigIsPermanent(t *testing.T) {
	for _, args := range []map[string]interface{}{
		{},
		{"connection": "proto://x/y"},
	} {
		tgt, _ := newTestTarget(t, args)
		err := tgt.Execute(message.Message{}, map[string]interface{}{
			"destination": "4242",
		})
		if err == nil || !errors.Is(err, asynq.SkipRetry) {
			t.Fatalf("args %v: expected permanent error, got %v", args, err)
		}
	}
}

func TestExecuteArgvAndLogs(t *testing.T) {
	requirePython(t)
	scriptPath, outPath := writeRecorder(t, 0)
	tgt, logs := newTestTarget(t, map[string]interface{}{
		"apprise":             scriptPath,
		"destination_pattern": telegramPattern,
		"connection":          `tgram://SYNTHETIC-BOT-SECRET/{{ arg "destination" }}`,
	})

	msg := message.Message{
		Title:   "SYNTHETIC TITLE",
		Message: "SYNTHETIC MESSAGE BODY",
	}
	err := tgt.Execute(msg, map[string]interface{}{"destination": "4242"})
	if err != nil {
		t.Fatal(err)
	}

	argvRaw, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatal(err)
	}
	argv := strings.Split(string(argvRaw), "\n")
	want := []string{
		"-vv",
		"-t", "SYNTHETIC TITLE",
		"-b", "SYNTHETIC MESSAGE BODY",
		"tgram://SYNTHETIC-BOT-SECRET/4242",
	}
	if len(argv) != len(want) {
		t.Fatalf("argv = %q, want %q", argv, want)
	}
	for i := range want {
		if argv[i] != want[i] {
			t.Fatalf("argv[%d] = %q, want %q", i, argv[i], want[i])
		}
	}

	for _, entry := range logs.All() {
		line := entry.Message + fmt.Sprint(entry.ContextMap())
		for _, secret := range []string{
			"SYNTHETIC-BOT-SECRET", "SYNTHETIC TITLE",
			"SYNTHETIC MESSAGE BODY", "4242", "tgram",
		} {
			if strings.Contains(line, secret) {
				t.Fatalf("log line %q contains %q", line, secret)
			}
		}
	}
}

func TestExecutePrefixDestination(t *testing.T) {
	requirePython(t)
	scriptPath, outPath := writeRecorder(t, 0)
	tgt, _ := newTestTarget(t, map[string]interface{}{
		"apprise":           scriptPath,
		"connection":        `proto://x/{{ arg "destination" }}`,
		"prefixDestination": true,
	})

	err := tgt.Execute(
		message.Message{Title: "title", Message: "body"},
		map[string]interface{}{"destination": "@someone"})
	if err != nil {
		t.Fatal(err)
	}

	argvRaw, _ := os.ReadFile(outPath)
	argv := strings.Split(string(argvRaw), "\n")
	if argv[2] != "@someone title" || argv[4] != "@someone body" {
		t.Fatalf("prefix missing in argv: %q", argv)
	}
}

func TestExecuteFailureLogsRedactedOutput(t *testing.T) {
	requirePython(t)
	scriptPath, _ := writeRecorder(t, 2)
	connection := `proto://bot:SYNTHETIC-CONNECTION-SECRET@host/4242`
	tgt, logs := newTestTarget(t, map[string]interface{}{
		"apprise":    scriptPath,
		"connection": strings.ReplaceAll(connection, "4242", `{{ arg "destination" }}`),
	})

	err := tgt.Execute(
		message.Message{Title: "t", Message: "m"},
		map[string]interface{}{"destination": "4242"})
	if err == nil {
		t.Fatal("expected error from failing recorder")
	}
	if errors.Is(err, asynq.SkipRetry) {
		t.Fatal("subprocess failure must stay retryable")
	}

	var failureLogged bool
	for _, entry := range logs.All() {
		line := entry.Message + fmt.Sprint(entry.ContextMap())
		if strings.Contains(line, "SYNTHETIC-CONNECTION-SECRET") {
			t.Fatalf("log line %q contains the connection string", line)
		}
		if strings.Contains(line, "recorder output") {
			failureLogged = true
			if !strings.Contains(line, "<connection>") {
				t.Fatalf("output logged without redaction marker: %q", line)
			}
		}
	}
	if !failureLogged {
		t.Fatal("failure output was not logged")
	}
}
