package cache

import (
	"context"
	"testing"
	"time"
)

func TestTryLockNilClient(t *testing.T) {
	_, ok, err := TryLock(context.Background(), nil, "k", time.Second)
	if err == nil || ok {
		t.Fatalf("expected nil client error, ok=%v err=%v", ok, err)
	}
}

func TestTryLockInvalidArgs(t *testing.T) {
	_, _, err := TryLock(context.Background(), nil, "", 0)
	if err == nil {
		t.Fatal("expected error")
	}
}
