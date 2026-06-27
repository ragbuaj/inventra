package storage

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"
)

func TestFakeRoundTrip(t *testing.T) {
	f := NewFake()
	ctx := context.Background()
	if err := f.Put(ctx, "k", bytes.NewReader([]byte("hi")), 2, "text/plain"); err != nil {
		t.Fatal(err)
	}
	rc, info, err := f.Get(ctx, "k")
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(rc)
	rc.Close()
	if string(b) != "hi" || info.ContentType != "text/plain" || info.Size != 2 {
		t.Fatalf("got %q %+v", b, info)
	}
	if !f.Has("k") {
		t.Fatal("Has should be true")
	}
	if err := f.Remove(ctx, "k"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := f.Get(ctx, "k"); !errors.Is(err, ErrObjectNotFound) {
		t.Fatalf("want ErrObjectNotFound, got %v", err)
	}
}
