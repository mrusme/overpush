package api

import (
	"context"
	"fmt"
	"io"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/mrusme/overpush/config"
	"github.com/mrusme/overpush/database"
	"github.com/mrusme/overpush/models/application"
	"github.com/mrusme/overpush/models/user"
	"github.com/mrusme/overpush/repositories"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

const (
	testUserKey   = "test-user-key-0000000000"
	testToken     = "test-token-pushover-0000"
	customToken   = "test-token-custom-000000"
	badTplToken   = "test-token-badtpl-000000"
	syntheticBody = "SYNTHETIC MESSAGE CONTENT"
)

func testConfig() *config.Config {
	cfg := &config.Config{}
	cfg.Testing = true
	cfg.Server.Enable = true
	cfg.Server.Limiter.MaxReqests = 100000
	cfg.Users = []user.User{
		{
			Enable: true,
			Key:    testUserKey,
			Applications: []application.Application{
				{
					Enable: true,
					Token:  testToken,
					Name:   "Test Pushover",
					Format: "pushover",
					Target: "no-such-target",
				},
				{
					Enable: true,
					Token:  customToken,
					Name:   "Test Custom",
					Format: "custom",
					Target: "no-such-target",
					CustomFormat: application.CFormat{
						Message: `{{ webhook "body.message" }}`,
						Title:   `{{ webhook "body.title" }}`,
					},
				},
				{
					Enable: true,
					Token:  badTplToken,
					Name:   "Test Bad Template",
					Format: "custom",
					Target: "no-such-target",
					CustomFormat: application.CFormat{
						Message: `{{ range 9 }}x{{ end }}`,
					},
				},
			},
		},
	}
	return cfg
}

func newTestAPI(t *testing.T, cfg *config.Config) (*API, *observer.ObservedLogs) {
	t.Helper()

	core, logs := observer.New(zap.DebugLevel)
	log := zap.New(core)

	a, err := New(cfg, log)
	if err != nil {
		t.Fatal(err)
	}
	if err := a.LoadMiddlewares(); err != nil {
		t.Fatal(err)
	}

	db, err := database.New(cfg, log)
	if err != nil {
		t.Fatal(err)
	}
	repos, err := repositories.New(cfg, db)
	if err != nil {
		t.Fatal(err)
	}
	a.repos = repos

	a.AttachRoutes()

	return a, logs
}

func request(
	t *testing.T,
	a *API,
	method string,
	path string,
	body string,
	headers map[string]string,
) (int, string) {
	t.Helper()
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, reader)
	if body != "" {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := a.app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(respBody)
}

func TestInternalGuard(t *testing.T) {
	a, _ := newTestAPI(t, testConfig())

	proxied := map[string]string{"X-Real-IP": "203.0.113.9"}
	forwarded := map[string]string{"X-Forwarded-For": "203.0.113.9"}

	blocked := []struct {
		method  string
		path    string
		headers map[string]string
	}{
		{"GET", "/_internal/health/livez", proxied},
		{"GET", "/_internal/health/livez", forwarded},
		{"GET", "/_internal/health/readyz", proxied},
		{"GET", "/_internal/health/startupz", proxied},
		{"POST", "/_internal/submit/" + testToken, proxied},
		{"POST", "/_internal/submit/" + testToken, forwarded},
		{"GET", "/_INTERNAL/health/livez", proxied},
		{"GET", "/_Internal/health/livez", proxied},
		{"GET", "/%5Finternal/health/livez", proxied},
		{"GET", "/_internal", proxied},
	}
	for _, tc := range blocked {
		status, body := request(t, a, tc.method, tc.path, "", tc.headers)
		if status != 404 {
			t.Fatalf("%s %s with %v: status %d body %q, want 404",
				tc.method, tc.path, tc.headers, status, body)
		}
	}

	status, body := request(t, a, "GET", "//_internal/health/livez", "", proxied)
	if status == 200 {
		t.Fatalf("//_internal with proxy header served: %d %q", status, body)
	}

	unblocked := []string{
		"/_internal/health/livez",
		"/_internal/health/readyz",
		"/_internal/health/startupz",
	}
	for _, path := range unblocked {
		status, body := request(t, a, "GET", path, "", nil)
		if status != 200 {
			t.Fatalf("GET %s without proxy headers: status %d body %q, want 200",
				path, status, body)
		}
	}
}

func TestInternalSubmitWithoutProxyHeaders(t *testing.T) {
	a, _ := newTestAPI(t, testConfig())

	status, body := request(t, a, "POST",
		"/_internal/submit/"+testToken,
		"title=msg.taxi confirmation code&message=Your code is 000000", nil)
	if status != 200 || !strings.Contains(body, `"status":1`) {
		t.Fatalf("internal submit: status %d body %q", status, body)
	}
}

func TestPublicRoutesUnaffectedByProxyHeaders(t *testing.T) {
	a, _ := newTestAPI(t, testConfig())

	status, body := request(t, a, "POST", "/1/messages.json",
		fmt.Sprintf("token=%s&user=%s&message=%s",
			testToken, testUserKey, syntheticBody),
		map[string]string{"X-Real-IP": "203.0.113.9"})
	if status != 200 || !strings.Contains(body, `"status":1`) {
		t.Fatalf("public submit with proxy header: status %d body %q",
			status, body)
	}
}

func TestCustomFormatSubmit(t *testing.T) {
	a, _ := newTestAPI(t, testConfig())

	req := httptest.NewRequest("POST", "/"+customToken,
		strings.NewReader(`{"message":"hello","title":"greeting"}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := a.app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 || !strings.Contains(string(body), `"status":1`) {
		t.Fatalf("custom submit: status %d body %q", resp.StatusCode, body)
	}
}

func TestCustomFormatRejectedTemplateNamesField(t *testing.T) {
	a, _ := newTestAPI(t, testConfig())

	req := httptest.NewRequest("POST", "/"+badTplToken,
		strings.NewReader(`{"message":"hello"}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := a.app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 400 {
		t.Fatalf("bad template: status %d body %q, want 400",
			resp.StatusCode, body)
	}
	if !strings.Contains(string(body), "custom format field Message") {
		t.Fatalf("bad template: body %q does not name the field", body)
	}
}

func TestRequestLogsCarryNoSecrets(t *testing.T) {
	a, logs := newTestAPI(t, testConfig())

	request(t, a, "POST", "/1/messages.json",
		fmt.Sprintf("token=%s&user=%s&message=%s&title=SYNTHETIC TITLE",
			testToken, testUserKey, syntheticBody), nil)

	var requestLogs int
	for _, entry := range logs.All() {
		line := entry.Message + fmt.Sprint(entry.ContextMap())
		for _, secret := range []string{
			testToken, testUserKey, syntheticBody, "SYNTHETIC TITLE",
		} {
			if strings.Contains(line, secret) {
				t.Fatalf("log line %q contains %q", line, secret)
			}
		}
		if entry.Message == "api.request" {
			requestLogs++
			ctx := entry.ContextMap()
			for _, field := range []string{
				"method", "route", "status", "duration", "request",
			} {
				if _, ok := ctx[field]; !ok {
					t.Fatalf("api.request log misses field %q: %v", field, ctx)
				}
			}
			if ctx["route"] != "/1/messages.json" {
				t.Fatalf("route field = %v", ctx["route"])
			}
		}
	}
	if requestLogs != 1 {
		t.Fatalf("got %d api.request entries, want 1", requestLogs)
	}
}

func TestTokenRouteLogsPatternNotToken(t *testing.T) {
	a, logs := newTestAPI(t, testConfig())

	req := httptest.NewRequest("POST", "/"+customToken,
		strings.NewReader(`{"message":"hi"}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := a.app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	for _, entry := range logs.All() {
		line := entry.Message + fmt.Sprint(entry.ContextMap())
		if strings.Contains(line, customToken) {
			t.Fatalf("log line %q contains the token", line)
		}
		if entry.Message == "api.request" {
			if entry.ContextMap()["route"] != "/:token" {
				t.Fatalf("route field = %v, want /:token",
					entry.ContextMap()["route"])
			}
		}
	}
}

func TestFormatInputAllowlist(t *testing.T) {
	headers := map[string][]string{
		"Content-Type":      {"application/json"},
		"User-Agent":        {"curl/8"},
		"Host":              {"via.example.com"},
		"X-Real-Ip":         {"203.0.113.9"},
		"X-Forwarded-For":   {"203.0.113.9"},
		"X-Forwarded-Proto": {"https"},
		"Authorization":     {"Bearer SYNTHETIC-AUTH-VALUE"},
		"Cookie":            {"session=SYNTHETIC-COOKIE"},
		"X-Api-Key":         {"SYNTHETIC-KEY"},
	}
	queries := map[string]string{"foo": "bar"}
	body := []byte(`{"message":"hello"}`)

	out := formatInput(headers, queries, body)

	for _, keep := range []string{
		"Content-Type: [application/json]",
		"User-Agent: [curl/8]",
		"Host: [via.example.com]",
		"foo: bar",
		`{"message":"hello"}`,
	} {
		if !strings.Contains(out, keep) {
			t.Fatalf("input %q misses %q", out, keep)
		}
	}
	for _, drop := range []string{
		"203.0.113.9", "SYNTHETIC-AUTH-VALUE", "SYNTHETIC-COOKIE",
		"SYNTHETIC-KEY", "X-Real-Ip", "X-Forwarded", "Authorization", "Cookie",
	} {
		if strings.Contains(out, drop) {
			t.Fatalf("input %q contains %q", out, drop)
		}
	}
}

func TestDisabledApplicationRejected(t *testing.T) {
	cfg := testConfig()
	cfg.Users[0].Applications[0].Enable = false
	a, _ := newTestAPI(t, cfg)

	status, body := request(t, a, "POST", "/1/messages.json",
		fmt.Sprintf("token=%s&user=%s&message=x", testToken, testUserKey), nil)
	if status != 404 {
		t.Fatalf("disabled app: status %d body %q, want 404", status, body)
	}
}

func TestDatabaseModeUnconfirmedRejected(t *testing.T) {
	pgurl := os.Getenv("OVERPUSH_TEST_PGURL")
	if pgurl == "" {
		t.Skip("OVERPUSH_TEST_PGURL not set")
	}

	const (
		userID           = "5f4664a2-70e9-4b58-8b0f-d1a17dbb2535"
		userKey          = "0e70ee49-6541-4b21-b3b9-6dbe1bde5182"
		confirmedToken   = "0cbe4f71-6d0a-47d6-a6f6-4d144e14e0b4"
		unconfirmedToken = "9e4f2b7e-2e64-42dd-8ba4-0439c31dbdcf"
	)

	ctx := context.Background()
	conn, err := pgx.Connect(ctx, pgurl)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)

	fixture, err := os.ReadFile("../database/testdata/fixture.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Exec(ctx, string(fixture)); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Exec(ctx,
		"INSERT INTO users (id, key, enable, created_at, updated_at) VALUES ($1, $2, true, now(), now())",
		userID, userKey); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Exec(ctx, `
INSERT INTO applications
  (id, user_id, target_id, enable, confirmed_at, token, name, format,
   created_at, updated_at)
VALUES
  ('a76a9f6e-1a93-40ff-99a5-9e2ef1379a34', $1,
   '59f2d250-1d2f-4efe-839c-edbee74bf929', true, now(), $2,
   'Confirmed App', 'pushover', now(), now()),
  ('7a4f2f68-91cb-45db-8a3e-53d33a3e0388', $1,
   '59f2d250-1d2f-4efe-839c-edbee74bf929', true, NULL, $3,
   'Unconfirmed App', 'pushover', now(), now())`,
		userID, confirmedToken, unconfirmedToken); err != nil {
		t.Fatal(err)
	}

	cfg := testConfig()
	cfg.Users = nil
	cfg.Database.Enable = true
	cfg.Database.Connection = pgurl
	a, _ := newTestAPI(t, cfg)

	status, body := request(t, a, "POST", "/1/messages.json",
		fmt.Sprintf("token=%s&user=%s&message=x", confirmedToken, userKey), nil)
	if status != 200 {
		t.Fatalf("confirmed app: status %d body %q, want 200", status, body)
	}

	status, body = request(t, a, "POST", "/1/messages.json",
		fmt.Sprintf("token=%s&user=%s&message=x", unconfirmedToken, userKey), nil)
	if status != 404 {
		t.Fatalf("unconfirmed app: status %d body %q, want 404", status, body)
	}

	status, body = request(t, a, "POST",
		"/_internal/submit/"+unconfirmedToken, "message=code delivery", nil)
	if status != 200 {
		t.Fatalf("internal submit to unconfirmed app: status %d body %q, want 200",
			status, body)
	}
}
