package main

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewGame(t *testing.T) {
	g := newGame()

	assert.Equal(t, "game", g.Name)
	assert.Equal(t, "GAME", g.EnvPrefix)
	assert.FileExists(t, filepath.Join(filepath.Dir(g.Source), "graphics", "project.godot"))
	assert.Equal(t, buildVersion, g.BuildVersion)
	assert.Equal(t, buildDate, g.BuildDate)
	assert.Equal(t, buildCommit, g.BuildCommit)
	require.NotNil(t, g.Register)
	require.NotNil(t, g.Setup)
	require.NotNil(t, g.Shutdown)
}
