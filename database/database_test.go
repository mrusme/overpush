package database

import (
	"context"
	"os"
	"testing"

	"github.com/mrusme/overpush/config"
	"go.uber.org/zap"
)

const (
	dbtUserID           = "5f4664a2-70e9-4b58-8b0f-d1a17dbb2535"
	dbtUserKey          = "0e70ee49-6541-4b21-b3b9-6dbe1bde5182"
	dbtTargetID         = "59f2d250-1d2f-4efe-839c-edbee74bf929"
	dbtConfirmedToken   = "0cbe4f71-6d0a-47d6-a6f6-4d144e14e0b4"
	dbtUnconfirmedToken = "9e4f2b7e-2e64-42dd-8ba4-0439c31dbdcf"
	dbtNonexistentToken = "00000000-0000-0000-0000-000000000000"
)

func newTestDatabase(t *testing.T) *Database {
	t.Helper()
	pgurl := os.Getenv("OVERPUSH_TEST_PGURL")
	if pgurl == "" {
		t.Skip("OVERPUSH_TEST_PGURL not set")
	}

	cfg := &config.Config{}
	cfg.Database.Enable = true
	cfg.Database.Connection = pgurl

	db, err := New(cfg, zap.NewNop())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Shutdown() })

	fixture, err := os.ReadFile("testdata/fixture.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.pool.Exec(context.Background(), string(fixture)); err != nil {
		t.Fatal(err)
	}

	seeds := []struct {
		sql  string
		args []interface{}
	}{
		{
			"INSERT INTO users (id, key, enable, created_at, updated_at) VALUES ($1, $2, true, now(), now())",
			[]interface{}{dbtUserID, dbtUserKey},
		},
		{
			`INSERT INTO targets (id, enable, type, name, created_at, updated_at, args) VALUES
			  ($1, true, 'apprise', 'Test Target', now(), now(),
			   '{"apprise": "/usr/bin/apprise", "connection": "proto://x/{{ arg \"destination\" }}"}')`,
			[]interface{}{dbtTargetID},
		},
		{
			`INSERT INTO applications
			  (id, user_id, target_id, enable, confirmed_at, token, name, format,
			   custom_format, target_args, created_at, updated_at)
			VALUES
			  ('a76a9f6e-1a93-40ff-99a5-9e2ef1379a34', $1, $2, true, now(), $3,
			   'Confirmed App', 'pushover', '{}',
			   '{"59f2d250-1d2f-4efe-839c-edbee74bf929": {"destination": "4242"}}',
			   now(), now()),
			  ('7a4f2f68-91cb-45db-8a3e-53d33a3e0388', $1, $2, true, NULL, $4,
			   'Unconfirmed App', 'custom', '{"Message": "{{ webhook \"body.message\" }}"}',
			   '{}', now(), now())`,
			[]interface{}{dbtUserID, dbtTargetID,
				dbtConfirmedToken, dbtUnconfirmedToken},
		},
	}
	for _, seed := range seeds {
		if _, err := db.pool.Exec(
			context.Background(), seed.sql, seed.args...); err != nil {
			t.Fatal(err)
		}
	}

	return db
}

func TestGetApplicationConfirmedFlag(t *testing.T) {
	db := newTestDatabase(t)

	app, err := db.GetApplication(dbtUserKey, dbtConfirmedToken)
	if err != nil {
		t.Fatal(err)
	}
	if !app.Enable || !app.Confirmed {
		t.Fatalf("confirmed app: enable=%v confirmed=%v", app.Enable, app.Confirmed)
	}
	if app.Name != "Confirmed App" || app.Format != "pushover" {
		t.Fatalf("field mapping: %+v", app)
	}
	if app.Target != dbtTargetID {
		t.Fatalf("target mapping: %q", app.Target)
	}
	nested, ok := app.TargetArgs[dbtTargetID].(map[string]interface{})
	if !ok || nested["destination"] != "4242" {
		t.Fatalf("target args mapping: %+v", app.TargetArgs)
	}

	app, err = db.GetApplication(dbtUserKey, dbtUnconfirmedToken)
	if err != nil {
		t.Fatal(err)
	}
	if app.Confirmed {
		t.Fatal("unconfirmed app reported as confirmed")
	}
	if app.CustomFormat.Message != `{{ webhook "body.message" }}` {
		t.Fatalf("custom format mapping: %+v", app.CustomFormat)
	}

	if _, err := db.GetApplication(dbtUserKey, dbtNonexistentToken); err == nil {
		t.Fatal("missing token: expected error")
	}
}

func TestGetUserFromToken(t *testing.T) {
	db := newTestDatabase(t)

	u, err := db.GetUserFromToken(dbtConfirmedToken)
	if err != nil {
		t.Fatal(err)
	}
	if u.Key != dbtUserKey || !u.Enable {
		t.Fatalf("user mapping: %+v", u)
	}
	if len(u.Applications) != 2 {
		t.Fatalf("applications: %d", len(u.Applications))
	}

	if _, err := db.GetUserFromToken(dbtNonexistentToken); err == nil {
		t.Fatal("missing token: expected error")
	}
}

func TestGetTargets(t *testing.T) {
	db := newTestDatabase(t)

	ts, err := db.GetTargets()
	if err != nil {
		t.Fatal(err)
	}
	if len(ts) != 1 || ts[0].ID != dbtTargetID || ts[0].Type != "apprise" {
		t.Fatalf("targets: %+v", ts)
	}

	tgt, err := db.GetTargetByID(dbtTargetID)
	if err != nil {
		t.Fatal(err)
	}
	if tgt.Args["apprise"] != "/usr/bin/apprise" {
		t.Fatalf("target args: %+v", tgt.Args)
	}
}

func TestIncrementStatAndSaveInput(t *testing.T) {
	db := newTestDatabase(t)

	if err := db.IncrementStat("", dbtConfirmedToken, "processed"); err != nil {
		t.Fatal(err)
	}
	if err := db.SaveInput("", dbtConfirmedToken, "input body"); err != nil {
		t.Fatal(err)
	}

	var processed, received int
	var input string
	if err := db.pool.QueryRow(context.Background(),
		"SELECT stat_processed, stat_received, latest_input FROM applications WHERE token = $1",
		dbtConfirmedToken).Scan(&processed, &received, &input); err != nil {
		t.Fatal(err)
	}
	if processed != 1 || received != 1 || input != "input body" {
		t.Fatalf("processed=%d received=%d input=%q", processed, received, input)
	}
}
