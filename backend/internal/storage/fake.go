package storage

import (
	"bytes"
	"context"
	"io"
	"sync"
)

type fakeObj struct {
	data        []byte
	contentType string
}

// Fake is an in-memory Storage for tests.
type Fake struct {
	mu   sync.Mutex
	objs map[string]fakeObj
	// PutErr, when set, makes the next Put fail (to test rollback paths).
	PutErr error
}

func NewFake() *Fake { return &Fake{objs: map[string]fakeObj{}} }

func (f *Fake) EnsureBucket(context.Context) error { return nil }

func (f *Fake) Put(_ context.Context, key string, r io.Reader, _ int64, ct string) error {
	if f.PutErr != nil {
		return f.PutErr
	}
	b, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.objs[key] = fakeObj{data: b, contentType: ct}
	return nil
}

func (f *Fake) Get(_ context.Context, key string) (io.ReadCloser, ObjectInfo, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	o, ok := f.objs[key]
	if !ok {
		return nil, ObjectInfo{}, ErrObjectNotFound
	}
	return io.NopCloser(bytes.NewReader(o.data)), ObjectInfo{ContentType: o.contentType, Size: int64(len(o.data))}, nil
}

func (f *Fake) Remove(_ context.Context, key string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.objs, key)
	return nil
}

// Has reports whether a key exists (test helper).
func (f *Fake) Has(key string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	_, ok := f.objs[key]
	return ok
}

// ObjsKeys returns all stored object keys (test helper for rollback assertions).
func (f *Fake) ObjsKeys() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	ks := make([]string, 0, len(f.objs))
	for k := range f.objs {
		ks = append(ks, k)
	}
	return ks
}
