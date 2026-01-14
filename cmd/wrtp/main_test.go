package main

import (
	"os"
	"testing"
)

func TestExists(t *testing.T) {
	// Test existing file
	tmpFile, err := os.CreateTemp("", "testfile")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())

	if !exists(tmpFile.Name()) {
		t.Errorf("Expected exists(%s) to be true", tmpFile.Name())
	}

	// Test non-existing file
	if exists("/path/to/non/existing/file/12345") {
		t.Error("Expected exists to be false for non-existing file")
	}
}
