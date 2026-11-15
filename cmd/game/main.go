package main

import (
	"runtime"

	"github.com/shigabutdinoff/go-godot-kit/pkg/kit"

	"github.com/shigabutdinoff/go-godot-tpl/internal/app"
)

var buildVersion, buildDate, buildCommit string

//go:generate go tool reset ../..

func main() {
	kit.Run(newGame())
}

func newGame() kit.Game {
	_, src, _, _ := runtime.Caller(0)
	a := app.New()
	return kit.Game{
		Name:      "game",
		EnvPrefix: "GAME",
		Source:    src,
		Register:  a.Register,
		Setup:     a.Setup,
		Shutdown:  a.Shutdown,

		BuildVersion: buildVersion, BuildDate: buildDate, BuildCommit: buildCommit,
	}
}
