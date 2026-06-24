package starter

import (
	"errors"
	"testing"
)

func TestCleanupStackReverseOrder(t *testing.T) {
	var order []int
	s := &CleanupStack{}
	s.Add(func() error { order = append(order, 1); return nil })
	s.Add(func() error { order = append(order, 2); return errors.New("boom") })
	err := s.Close()
	if err == nil {
		t.Fatal("expected joined error")
	}
	if len(order) != 2 || order[0] != 2 || order[1] != 1 {
		t.Fatalf("unexpected order: %#v", order)
	}
}
