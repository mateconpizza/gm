package db

import (
	"errors"
	"testing"
)

func TestDropRepository(t *testing.T) {
	t.Parallel()
	const n = 10
	r := testPopulatedDB(t, n)
	defer teardownthewall(r.DB)

	b, err := r.ByID(t.Context(), 1)
	if err != nil {
		t.Fatalf("unexpected error retrieving bookmark: %v", err)
	}
	if b == nil {
		t.Fatal("expected bookmark to exist, got nil")
	}

	err = drop(t.Context(), r)
	if err != nil {
		t.Fatalf("failed to drop repository: %v", err)
	}

	b, err = r.ByID(t.Context(), 1)
	if b != nil {
		t.Errorf("expected nil bookmark after drop, got: %+v", b)
	}
	if err == nil {
		t.Fatal("expected error after drop, got nil")
	}
	if !errors.Is(err, ErrRecordNotFound) {
		t.Errorf("expected error to contain %q, got %q", ErrRecordNotFound.Error(), err.Error())
	}
}
