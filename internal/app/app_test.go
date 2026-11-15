package app

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestApp_Shutdown_Hooks(t *testing.T) {
	errSave := errors.New("save failed")
	errLoad := errors.New("load failed")

	tests := []struct {
		name    string
		fails   []error
		wantErr error
	}{
		{name: "no errors"},
		{name: "one error", fails: []error{errSave}, wantErr: errSave},
		{name: "first error wins", fails: []error{errSave, errLoad}, wantErr: errSave},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			a := New()
			ctx, cancel := context.WithCancel(t.Context())
			cancel()

			a.Register(ctx)
			a.Setup(ctx)
			for _, err := range tt.fails {
				a.Fail(ctx, err)
			}
			err := a.Shutdown(ctx)
			if tt.wantErr == nil {
				require.NoError(t, err)
				return
			}
			require.ErrorIs(t, err, tt.wantErr)
		})
	}
}

func waitCancel(stopped *atomic.Bool) Worker {
	return func(ctx context.Context) error {
		<-ctx.Done()
		stopped.Store(true)
		return nil
	}
}

func TestApp_Shutdown_Errors(t *testing.T) {
	errWorld := errors.New("world failed")
	errClasses := errors.New("classes failed")
	var stopped atomic.Bool

	tests := []struct {
		name            string
		workers         []Worker
		fail            error
		failBeforeSetup bool
		setup           bool
		wantErr         error
		wantMsg         string
		wantStopped     bool
	}{
		{name: "fail stops workers", workers: []Worker{waitCancel(&stopped)}, fail: errWorld, setup: true,
			wantErr: errWorld, wantStopped: true},
		{name: "worker error stops others", setup: true, wantErr: errWorld, wantMsg: "worker 0: world failed",
			workers: []Worker{func(context.Context) error { return errWorld }, waitCancel(&stopped)}, wantStopped: true},
		{name: "fail before setup", workers: []Worker{waitCancel(&stopped)}, fail: errClasses, failBeforeSetup: true,
			setup: true, wantErr: errClasses, wantStopped: true},
		{name: "without setup"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stopped.Store(false)
			a := New(tt.workers...)
			a.Register(t.Context())
			if tt.failBeforeSetup {
				a.Fail(t.Context(), tt.fail)
			}
			if tt.setup {
				a.Setup(t.Context())
			}
			if !tt.failBeforeSetup && tt.fail != nil {
				a.Fail(t.Context(), tt.fail)
			}

			err := a.Shutdown(t.Context())
			if tt.wantErr == nil {
				require.NoError(t, err)
			} else {
				require.ErrorIs(t, err, tt.wantErr)
			}
			if tt.wantMsg != "" {
				assert.EqualError(t, err, tt.wantMsg)
			}
			assert.Equal(t, tt.wantStopped, stopped.Load())
		})
	}
}

func TestApp_Fail_Concurrent(t *testing.T) {
	a := New()
	a.Register(t.Context())
	a.Setup(t.Context())
	var wg sync.WaitGroup
	for range 10 {
		wg.Go(func() { a.Fail(t.Context(), errors.New("worker failed")) })
	}
	wg.Wait()

	assert.EqualError(t, a.Shutdown(t.Context()), "worker failed")
}

func TestApp_Shutdown_WorkerStop(t *testing.T) {
	errWorld := errors.New("world failed")

	tests := []struct {
		name    string
		worker  Worker
		wantErr error
	}{
		{name: "nil on ctx", worker: func(ctx context.Context) error { <-ctx.Done(); return nil }},
		{name: "ctx.Err on ctx", worker: func(ctx context.Context) error { <-ctx.Done(); return ctx.Err() }},
		{name: "wrapped ctx.Err on ctx", worker: func(ctx context.Context) error {
			<-ctx.Done()
			return fmt.Errorf("autosave: %w", ctx.Err())
		}},
		{name: "error on ctx", worker: func(ctx context.Context) error { <-ctx.Done(); return errWorld }, wantErr: errWorld},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			a := New(tt.worker)
			a.Register(t.Context())
			a.Setup(t.Context())

			err := a.Shutdown(t.Context())
			if tt.wantErr == nil {
				require.NoError(t, err)
				return
			}
			require.ErrorIs(t, err, tt.wantErr)
		})
	}
}

func TestApp_Setup_WorkerOwnCanceled(t *testing.T) {
	stopped := make(chan struct{})
	own := func(ctx context.Context) error {
		ctx, cancel := context.WithCancel(ctx)
		cancel()
		return ctx.Err()
	}
	a := New(own, func(ctx context.Context) error { <-ctx.Done(); close(stopped); return nil })
	a.Register(t.Context())
	a.Setup(t.Context())

	select {
	case <-stopped:
	case <-time.After(5 * time.Second):
		t.Fatal("context.Canceled of the worker's own did not stop the other workers")
	}
	require.ErrorIs(t, a.Shutdown(t.Context()), context.Canceled)
}

func TestApp_Shutdown_ExpiredStopped(t *testing.T) {
	expired, cancel := context.WithCancel(t.Context())
	cancel()
	for range 100 {
		a := New(func(context.Context) error { return nil }, func(context.Context) error { return nil })
		a.Register(t.Context())
		a.Setup(t.Context())
		require.Eventually(t, func() bool { return len(a.returned) == 2 }, 5*time.Second, time.Millisecond)

		require.NoError(t, a.Shutdown(expired))
	}
}

func TestApp_Shutdown_Timeout(t *testing.T) {
	errWorld := errors.New("world failed")
	release := make(chan struct{})
	defer close(release)
	a := New(func(context.Context) error { <-release; return nil })
	a.Register(t.Context())
	a.Setup(t.Context())
	a.Fail(t.Context(), errWorld)

	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Millisecond)
	defer cancel()
	err := a.Shutdown(ctx)

	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.ErrorIs(t, err, errWorld)
	assert.ErrorContains(t, err, "wait for workers")
}

func TestApp_Fail_WorkerErrorLogged(t *testing.T) {
	errWorld := errors.New("world failed")
	errSave := errors.New("save failed")
	var logs bytes.Buffer
	a := New(func(ctx context.Context) error { <-ctx.Done(); return errSave })
	a.Register(t.Context())
	a.log = slog.New(slog.NewJSONHandler(&logs, nil))
	a.Setup(t.Context())

	a.Fail(t.Context(), errWorld)

	require.ErrorIs(t, a.Shutdown(t.Context()), errWorld)
	assert.Contains(t, logs.String(), `"exception.message":"world failed"`)
	assert.Contains(t, logs.String(), `"exception.message":"worker 0: save failed"`)
}

func BenchmarkFail(b *testing.B) {
	a := New()
	a.log = slog.New(slog.NewJSONHandler(io.Discard, nil))
	a.Setup(b.Context())
	err := errors.New("worker failed")
	for b.Loop() {
		a.Fail(b.Context(), err)
	}
}

func BenchmarkLifecycle(b *testing.B) {
	workers := make([]Worker, 8)
	for i := range workers {
		workers[i] = func(ctx context.Context) error { <-ctx.Done(); return ctx.Err() }
	}
	log := slog.New(slog.NewJSONHandler(io.Discard, nil))
	for b.Loop() {
		a := New(workers...)
		a.log = log
		a.Setup(b.Context())
		if err := a.Shutdown(b.Context()); err != nil {
			b.Fatal(err)
		}
	}
}
