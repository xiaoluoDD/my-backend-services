package db

import (
	"path/filepath"
	"testing"
)

func TestHashPasswordRoundTrip(t *testing.T) {
	hash, err := HashPassword("secret")
	if err != nil {
		t.Fatal(err)
	}
	if !VerifyPassword("secret", hash) {
		t.Fatal("expected password to verify")
	}
	if VerifyPassword("wrong", hash) {
		t.Fatal("wrong password should not verify")
	}
}

func TestAccountLoginLogs(t *testing.T) {
	dir := t.TempDir()
	dbConn, err := Open(filepath.Join(dir, "login.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer dbConn.Close()

	if err := RecordAccountLogin(dbConn, "alice"); err != nil {
		t.Fatal(err)
	}
	if err := RecordAccountLogin(dbConn, "alice"); err != nil {
		t.Fatal(err)
	}
	if err := RecordAccountLogin(dbConn, "bob"); err != nil {
		t.Fatal(err)
	}

	list, err := ListRecentAccountLogins(dbConn, "alice", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 {
		t.Fatalf("want 2 alice logs, got %d", len(list))
	}
	list, err = ListRecentAccountLogins(dbConn, "bob", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 {
		t.Fatalf("want 1 bob log, got %d", len(list))
	}
}
