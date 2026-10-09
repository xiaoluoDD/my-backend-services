package db

import (
	"path/filepath"
	"testing"
)

func TestDeepseekAPIKeyRoundTrip(t *testing.T) {
	sqlDB, err := Open(filepath.Join(t.TempDir(), "assistant.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()

	got, err := GetDeepseekAPIKey(sqlDB)
	if err != nil {
		t.Fatal(err)
	}
	if got != "" {
		t.Fatalf("empty db key=%q", got)
	}

	if err := SaveDeepseekAPIKey(sqlDB, " sk-test-12345678 "); err != nil {
		t.Fatal(err)
	}
	got, err = GetDeepseekAPIKey(sqlDB)
	if err != nil {
		t.Fatal(err)
	}
	if got != "sk-test-12345678" {
		t.Fatalf("saved key=%q", got)
	}
	if mask := MaskSecret(got); mask != "sk-****5678" {
		t.Fatalf("mask=%q", mask)
	}

	if err := SaveDeepseekAPIKey(sqlDB, ""); err != nil {
		t.Fatal(err)
	}
	got, err = GetDeepseekAPIKey(sqlDB)
	if err != nil {
		t.Fatal(err)
	}
	if got != "" {
		t.Fatalf("cleared key=%q", got)
	}
}
