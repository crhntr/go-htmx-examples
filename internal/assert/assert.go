package assert

import "testing"

func Equal[T comparable](t *testing.T, exp, got T) {
	t.Helper()
	if exp != got {
		t.Errorf("expected %v, got %v", exp, got)
	}
}

func Zero[T comparable](t *testing.T, got T) {
	t.Helper()
	var zero T
	if got != zero {
		t.Errorf("expected zero, got %v", got)
	}
}

func True(t *testing.T, got bool, msg ...string) {
	t.Helper()
	if !got {
		for _, m := range msg {
			t.Log(m)
		}
		t.Errorf("expected true, got false")
	}
}

func Panics(t *testing.T, f func()) {
	t.Helper()
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("expected panic, got nil")
		}
	}()
	f()
}

func NotPanics(t *testing.T, f func()) {
	t.Helper()
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("expected no panic, got %v", r)
		}
	}()
	f()
}
