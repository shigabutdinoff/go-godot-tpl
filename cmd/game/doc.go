// Game is an empty 3D game on go-godot-kit. Go embeds the Godot engine through
// LibGodot (graphics.gd). The scene and assets live in cmd/game/graphics.
//
// Usage:
//
//	go run ./cmd/game [flags] [-- Godot arguments]
//
// Flags (an env variable overrides a flag, a flag overrides the JSON file
// from -c/-config/CONFIG/GAME_CONFIG):
//
//	-version            print the version and exit
//	-project <dir>      Godot project (GAME_PROJECT)
//	-log-level <level>  debug, info, warn, error (GAME_LOG_LEVEL)
//	-log-file <file>    write logs to a file instead of stderr (GAME_LOG_FILE)
//	-cpuprofile <file>  CPU profile (GAME_CPU_PROFILE)
//	-memprofile <file>  memory profile on exit (GAME_MEM_PROFILE)
//	-c, -config <file>  JSON settings file
//
// The game runs on macOS, Linux and Windows, arm64 and amd64. Development
// builds load LibGodot from lib/ at the project root, and go tool libgodot
// builds it. Releases built by gd embed the engine and the scene.
//
// Exit codes: 0 on success or -h, 1 on error, 2 on invalid arguments.
//
// The game exits on SIGINT, SIGTERM and SIGQUIT. On Windows these are Ctrl+C,
// Ctrl+Break and closing the console. The game stops the workers and saves
// data within 5 s. A second signal kills the process.
//
// Copyright (c) 2026 Bulat Shigabutdinov. MIT License, see LICENSE.
package main
