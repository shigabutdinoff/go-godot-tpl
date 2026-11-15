package app_test

import (
	"context"
	"errors"
	"fmt"

	"github.com/shigabutdinoff/go-godot-tpl/internal/app"
)

// Example shows the hooks in the order kit.Run calls them. One worker fails,
// the other stops on its ctx, and Shutdown returns the error. The game would
// exit with code 1.
func Example() {
	errAutosave := errors.New("autosave failed")
	world := func(ctx context.Context) error {
		<-ctx.Done()
		return nil
	}
	autosave := func(context.Context) error { return errAutosave }

	a := app.New(world, autosave)
	ctx := context.Background()
	a.Register(ctx)
	a.Setup(ctx)
	err := a.Shutdown(ctx)

	fmt.Println(errors.Is(err, errAutosave))
	// Output: true
}

// ExampleApp_Fail shows a hook failing the game. Fail stops the workers, and
// Shutdown returns the error, so the game would exit with code 1.
func ExampleApp_Fail() {
	errSave := errors.New("save not found")
	a := app.New()
	ctx := context.Background()
	a.Register(ctx)
	a.Setup(ctx)
	a.Fail(ctx, fmt.Errorf("load world: %w", errSave))
	err := a.Shutdown(ctx)

	fmt.Println(err)
	// Output: load world: save not found
}
