// Package app holds the game's dependencies and state and the hooks that
// kit.Run calls. Register runs before the scene loads, Setup after it loads,
// and Shutdown after the engine stops.
//
// Copyright (c) 2026 Bulat Shigabutdinov. MIT License, see LICENSE.
package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
)

// Worker is a game goroutine, such as a world simulation or autosave. It must
// return once ctx is canceled. Returning nil is a normal stop. After ctx is
// canceled, so is returning ctx.Err() or any error that wraps context.Canceled.
//
// Any other error goes to [App.Fail] as "worker N: …", where N is the
// worker's index in [New]. So does context.Canceled while ctx is still alive,
// for example from a context the worker made itself.
type Worker func(ctx context.Context) error

// App is the game. Create it with [New] and pass its methods to kit.Game.
//
// Setup starts a goroutine that owns the workers' ctx. It cancels the ctx on
// the first Fail and on Shutdown. The goroutines share no memory and talk
// through channels.
type App struct {
	workers []Worker
	log     *slog.Logger

	first    chan error
	failed   chan error
	stop     chan struct{}
	started  chan struct{}
	returned chan struct{}
}

// New returns a game that runs the given workers. Setup starts them.
func New(workers ...Worker) *App {
	return &App{
		workers:  workers,
		log:      slog.Default(),
		first:    make(chan error, 1),
		failed:   make(chan error, 1),
		stop:     make(chan struct{}, 1),
		started:  make(chan struct{}),
		returned: make(chan struct{}, len(workers)),
	}
}

// Register runs before the scene loads. Register your Godot classes here.
// ctx is canceled on an OS quit signal.
func (a *App) Register(ctx context.Context) {
	a.log = slog.With("component", "game")
	a.log.DebugContext(ctx, "classes registered")
}

// Setup runs once the scene is loaded. Build the world here. kit calls it once.
//
// Setup starts the workers and returns without waiting for them. Their ctx is
// canceled by Fail, by the first error a worker returns, by an OS quit signal
// and by Shutdown. kit always calls Shutdown after Setup.
//
// Setup has no error result. On a fatal error, call Fail and quit the scene
// tree, and the game exits with code 1.
func (a *App) Setup(ctx context.Context) {
	close(a.started)
	go a.run(ctx)
	a.log.DebugContext(ctx, "scene ready", "custom.workers", len(a.workers))
}

// Shutdown runs after the engine stops. Save data here, within 5 s. Shutdown
// stops the workers and waits for them. kit calls it once.
//
// It returns the first error passed to Fail, worker errors included. If ctx
// expires before the workers stop, Shutdown stops waiting and joins ctx.Err()
// to that error.
func (a *App) Shutdown(ctx context.Context) error {
	var timeout error
	select {
	case <-a.started:
		a.stop <- struct{}{}
		if n := a.wait(ctx); n > 0 {
			timeout = fmt.Errorf("wait for workers: %w", ctx.Err())
			a.log.ErrorContext(ctx, "workers did not stop", "exception.message", timeout, "custom.workers", n)
		}
	default:
	}
	if timeout == nil {
		a.log.DebugContext(ctx, "game stopped")
	}
	var err error
	select {
	case err = <-a.first:
	default:
	}
	return errors.Join(err, timeout)
}

// Fail logs a fatal error and stops the workers. Shutdown returns the first
// error passed to Fail. Hooks and workers may call Fail. It never blocks.
func (a *App) Fail(ctx context.Context, err error) {
	a.log.ErrorContext(ctx, "game failed", "exception.message", err)
	select {
	case a.first <- err:
	default:
	}
	select {
	case a.failed <- err:
	default:
	}
}

func (a *App) run(ctx context.Context) {
	ctx, cancel := context.WithCancelCause(ctx)
	defer cancel(context.Canceled)
	for i, w := range a.workers {
		go func() {
			defer func() { a.returned <- struct{}{} }()
			err := w(ctx)
			if err == nil || errors.Is(err, context.Canceled) && ctx.Err() != nil {
				return
			}
			a.Fail(ctx, fmt.Errorf("worker %d: %w", i, err))
		}()
	}
	for {
		select {
		case err := <-a.failed:
			cancel(err)
		case <-a.stop:
			return
		}
	}
}

func (a *App) wait(ctx context.Context) int {
	n := len(a.workers)
	for n > 0 {
		select {
		case <-a.returned:
			n--
		case <-ctx.Done():
			for n > 0 {
				select {
				case <-a.returned:
					n--
				default:
					return n
				}
			}
		}
	}
	return n
}
