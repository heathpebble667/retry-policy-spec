package main

import (
	"os"
	"path/filepath"
	"testing"
)

const canonicalPolicy = "max_attempts: 3\nbackoff: fixed delay=1s\njitter: none\n"

const messyPolicy = "max_attempts:   3\n" +
	"backoff:   fixed   delay=1s\n"

func writeTemp(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing temp file: %v", err)
	}
	return path
}

func TestProcessFileWriteRewritesToCanonical(t *testing.T) {
	dir := t.TempDir()
	path := writeTemp(t, dir, "policy.txt", messyPolicy)

	if err := processFile(path, false, true); err != nil {
		t.Fatalf("processFile: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading back: %v", err)
	}
	if string(got) != canonicalPolicy {
		t.Fatalf("processFile -w left %q, want %q", got, canonicalPolicy)
	}
}

func TestProcessFileWriteLeavesCanonicalFileAlone(t *testing.T) {
	dir := t.TempDir()
	path := writeTemp(t, dir, "policy.txt", canonicalPolicy)

	if err := processFile(path, false, true); err != nil {
		t.Fatalf("processFile: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading back: %v", err)
	}
	if string(got) != canonicalPolicy {
		t.Fatalf("processFile -w touched an already-canonical file: got %q", got)
	}
}

func TestProcessFileListDoesNotWrite(t *testing.T) {
	dir := t.TempDir()
	path := writeTemp(t, dir, "policy.txt", messyPolicy)

	if err := processFile(path, true, false); err != nil {
		t.Fatalf("processFile: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading back: %v", err)
	}
	if string(got) != messyPolicy {
		t.Fatalf("processFile -l modified the file: got %q, want unchanged %q", got, messyPolicy)
	}
}

func TestProcessFileInvalidPolicyReturnsError(t *testing.T) {
	dir := t.TempDir()
	path := writeTemp(t, dir, "policy.txt", "max_attempts: 0\nbackoff: fixed delay=1s\n")

	if err := processFile(path, false, false); err == nil {
		t.Fatal("processFile: expected an error for an invalid policy, got nil")
	}
}

func TestProcessFileMissingFileReturnsError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "does-not-exist.txt")

	if err := processFile(path, false, false); err == nil {
		t.Fatal("processFile: expected an error for a missing file, got nil")
	}
}
