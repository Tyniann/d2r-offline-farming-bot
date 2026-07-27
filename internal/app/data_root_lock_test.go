package app

import (
	"errors"
	"testing"
)

func TestDataRootLockRejectsSecondCoreAndReleases(t *testing.T) {
	root := t.TempDir()
	first, err := AcquireDataRootLock(root)
	if err != nil {
		t.Fatal(err)
	}
	defer first.Close()
	_, err = AcquireDataRootLock(root)
	var rootErr *DataRootError
	if !errors.As(err, &rootErr) || rootErr.Code != Phase15ReasonDataRootLocked {
		t.Fatalf("second lock err=%v", err)
	}
	if closeErr := first.Close(); closeErr != nil {
		t.Fatal(closeErr)
	}
	third, err := AcquireDataRootLock(root)
	if err != nil {
		t.Fatalf("lock after release: %v", err)
	}
	if closeErr := third.Close(); closeErr != nil {
		t.Fatal(closeErr)
	}
}
