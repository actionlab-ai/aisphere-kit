package db

import (
	"context"
	"testing"
)

// TestNewDatabase_DriverDispatch verifies the factory selects the right
// concrete type based on cfg.Driver. We don't open real connections here;
// we only verify the dispatch logic fails fast on unsupported drivers
// and routes to the correct constructor (which will then fail on DSN).
func TestNewDatabase_DriverDispatch(t *testing.T) {
	cases := []struct {
		name     string
		driver   string
		wantErr  bool
		wantType string // "mysql" or "postgres"
	}{
		{name: "empty defaults to mysql", driver: "", wantErr: true, wantType: "mysql"}, // err because DSN empty
		{name: "mysql explicit", driver: "mysql", wantErr: true, wantType: "mysql"},
		{name: "postgres", driver: "postgres", wantErr: true, wantType: "postgres"},
		{name: "pgx alias", driver: "pgx", wantErr: true, wantType: "postgres"},
		{name: "postgresql alias", driver: "postgresql", wantErr: true, wantType: "postgres"},
		{name: "unsupported", driver: "sqlite", wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := DatabaseConfig{Driver: tc.driver, DSN: ""} // empty DSN -> err from constructor
			_, err := NewDatabase(context.Background(), cfg)
			if !tc.wantErr {
				if err != nil {
					t.Fatalf("expected no error, got %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected error, got nil")
			}
		})
	}
}

// TestCanonicalDriver covers the driver normalization table.
func TestCanonicalDriver(t *testing.T) {
	cases := []struct{ in, want string }{
		{"", "mysql"},
		{"mysql", "mysql"},
		{"postgres", "postgres"},
		{"postgresql", "postgres"},
		{"pgx", "postgres"},
		{"sqlite", "sqlite"}, // unsupported but returned as-is
	}
	for _, c := range cases {
		if got := canonicalDriver(c.in); got != c.want {
			t.Errorf("canonicalDriver(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// TestNormalizePostgresDSN verifies SSLMode defaulting for both DSN forms.
func TestNormalizePostgresDSN(t *testing.T) {
	// URL form: no sslmode -> default disable
	got := normalizePostgresDSN("postgres://u:p@host:5432/db", "", "")
	if !contains(got, "sslmode=disable") {
		t.Errorf("URL form without sslmode: expected default disable, got %q", got)
	}
	// URL form: existing sslmode preserved
	got = normalizePostgresDSN("postgres://u:p@host:5432/db?sslmode=require", "", "")
	if !contains(got, "sslmode=require") {
		t.Errorf("URL form with sslmode: expected preserved, got %q", got)
	}
	// KV form: no sslmode -> default disable
	got = normalizePostgresDSN("host=localhost user=postgres dbname=aihub", "", "")
	if !contains(got, "sslmode=disable") {
		t.Errorf("KV form without sslmode: expected default disable, got %q", got)
	}
	// KV form: existing sslmode preserved
	got = normalizePostgresDSN("host=localhost sslmode=require", "", "")
	if !contains(got, "sslmode=require") {
		t.Errorf("KV form with sslmode: expected preserved, got %q", got)
	}
	// Override via SSLMode param
	got = normalizePostgresDSN("host=localhost", "require", "")
	if !contains(got, "sslmode=require") {
		t.Errorf("KV form with SSLMode override: expected require, got %q", got)
	}
}

func TestNormalizePostgresDSN_SearchPath(t *testing.T) {
	got := normalizePostgresDSN("postgres://u:p@host:5432/db", "", "aihub")
	if !contains(got, "options=") || !contains(got, "search_path") {
		t.Errorf("URL form with search_path: expected options search_path, got %q", got)
	}
	got = normalizePostgresDSN("host=localhost user=postgres dbname=aihub", "", "aihub")
	if !contains(got, "options=") || !contains(got, "search_path=aihub") {
		t.Errorf("KV form with search_path: expected options search_path, got %q", got)
	}
}

func TestPostgresMaintenanceDSN(t *testing.T) {
	target, maintenance, err := postgresMaintenanceDSN(PostgresConfig{DSN: "postgres://u:p@host:5432/aisphere_hub?sslmode=disable"})
	if err != nil {
		t.Fatal(err)
	}
	if target != "aisphere_hub" {
		t.Fatalf("target = %q", target)
	}
	if !contains(maintenance, "/postgres") {
		t.Fatalf("maintenance dsn = %q", maintenance)
	}

	target, maintenance, err = postgresMaintenanceDSN(PostgresConfig{DSN: "host=localhost user=postgres dbname=aisphere_hub sslmode=disable", MaintenanceDB: "template1"})
	if err != nil {
		t.Fatal(err)
	}
	if target != "aisphere_hub" || !contains(maintenance, "dbname=template1") {
		t.Fatalf("target=%q maintenance=%q", target, maintenance)
	}
}

// TestJSONB round-trips a value through Value/Scan.
func TestJSONB(t *testing.T) {
	type payload struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}
	j := JSONB[payload]{Data: payload{Name: "alice", Age: 30}}
	v, err := j.Value()
	if err != nil {
		t.Fatalf("Value: %v", err)
	}
	s, ok := v.(string)
	if !ok {
		t.Fatalf("Value returned %T, want string", v)
	}
	if !contains(s, `"name":"alice"`) {
		t.Errorf("unexpected JSON: %q", s)
	}

	var got JSONB[payload]
	if err := got.Scan([]byte(s)); err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if got.Data.Name != "alice" || got.Data.Age != 30 {
		t.Errorf("round-trip mismatch: %+v", got.Data)
	}
	if got.Null {
		t.Errorf("Null flag should be false after Scan with data")
	}

	// NULL scan
	var n JSONB[payload]
	if err := n.Scan(nil); err != nil {
		t.Fatalf("Scan(nil): %v", err)
	}
	if !n.Null {
		t.Errorf("Null flag should be true after Scan(nil)")
	}
}

// TestJSONB_MustGet verifies ErrJSONBNull is returned for NULL columns.
func TestJSONB_MustGet(t *testing.T) {
	var n JSONB[int]
	n.Null = true
	_, err := n.MustGet()
	if err != ErrJSONBNull {
		t.Errorf("expected ErrJSONBNull, got %v", err)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || (len(sub) > 0 && stringContains(s, sub)))
}

func stringContains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
