package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestAskedOncePerBuild(t *testing.T) {
	dir := t.TempDir()
	ledger := filepath.Join(dir, "nested", "tcc-asked.json")
	exe := filepath.Join(dir, "kinclaw")
	if err := os.WriteFile(exe, []byte("build one"), 0o755); err != nil {
		t.Fatal(err)
	}

	first := buildStamp(exe)
	if first == "" {
		t.Fatal("no stamp for a binary that exists")
	}
	if askedAlready(ledger, "accessibility", first) {
		t.Fatal("a build that never asked reads as having asked")
	}
	rememberAsked(ledger, "accessibility", first)
	if !askedAlready(ledger, "accessibility", first) {
		t.Fatal("the same build would ask twice")
	}
	if askedAlready(ledger, "screen", first) {
		t.Fatal("asking for one permission silenced another")
	}

	// A rebuild: new bytes, new mtime — the grant is stale, so it may ask.
	if err := os.WriteFile(exe, []byte("build two, longer"), 0o755); err != nil {
		t.Fatal(err)
	}
	later := time.Now().Add(time.Minute)
	if err := os.Chtimes(exe, later, later); err != nil {
		t.Fatal(err)
	}
	second := buildStamp(exe)
	if second == first {
		t.Fatal("a rebuild kept the old stamp")
	}
	if askedAlready(ledger, "accessibility", second) {
		t.Fatal("a rebuilt binary was not allowed its one dialog")
	}
}

func TestAskedFailsTowardAsking(t *testing.T) {
	dir := t.TempDir()
	ledger := filepath.Join(dir, "tcc-asked.json")

	if buildStamp(filepath.Join(dir, "missing")) != "" {
		t.Fatal("a missing binary got a stamp")
	}
	// No stamp, no ledger: never claims to have asked, never writes.
	rememberAsked(ledger, "accessibility", "")
	rememberAsked("", "accessibility", "stamp")
	if _, err := os.Stat(ledger); err == nil {
		t.Fatal("wrote a ledger entry without a stamp")
	}
	if askedAlready("", "accessibility", "stamp") || askedAlready(ledger, "accessibility", "") {
		t.Fatal("claimed to have asked with nothing to go on")
	}

	// A corrupt ledger is an empty one.
	if err := os.WriteFile(ledger, []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if askedAlready(ledger, "accessibility", "stamp") {
		t.Fatal("a corrupt ledger claimed to have asked")
	}
	rememberAsked(ledger, "accessibility", "stamp")
	if !askedAlready(ledger, "accessibility", "stamp") {
		t.Fatal("could not recover from a corrupt ledger")
	}
}
