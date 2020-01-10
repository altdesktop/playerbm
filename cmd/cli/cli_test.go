package cli

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestCliGoodPath(t *testing.T) {
	var cli *PbmCli
	var err error

	cli, err = ParseArgs([]string{"playerbm"})
	require.NoError(t, err)
	require.True(t, cli.HelpFlag)

	cli, err = ParseArgs([]string{"playerbm", "-h"})
	require.NoError(t, err)
	require.True(t, cli.HelpFlag)

	cli, err = ParseArgs([]string{"playerbm", "--help"})
	require.NoError(t, err)
	require.True(t, cli.HelpFlag)

	cli, err = ParseArgs([]string{"playerbm", "-v"})
	require.NoError(t, err)
	require.True(t, cli.VersionFlag)

	cli, err = ParseArgs([]string{"playerbm", "--version"})
	require.NoError(t, err)
	require.True(t, cli.VersionFlag)

	cli, err = ParseArgs([]string{"playerbm", "--list-bookmarks"})
	require.NoError(t, err)
	require.True(t, cli.ListBookmarksFlag)

	cli, err = ParseArgs([]string{"playerbm", "-l"})
	require.NoError(t, err)
	require.True(t, cli.ListBookmarksFlag)

	cli, err = ParseArgs([]string{"playerbm", "--list-players"})
	require.NoError(t, err)
	require.True(t, cli.ListPlayersFlag)

	cli, err = ParseArgs([]string{"playerbm", "-L"})
	require.NoError(t, err)
	require.True(t, cli.ListPlayersFlag)

	cli, err = ParseArgs([]string{"playerbm", "--save"})
	require.NoError(t, err)
	require.True(t, cli.SaveFlag)

	cli, err = ParseArgs([]string{"playerbm", "-s"})
	require.NoError(t, err)
	require.True(t, cli.SaveFlag)

	cli, err = ParseArgs([]string{"playerbm", "--save", "mpv"})
	require.NoError(t, err)
	require.True(t, cli.SaveFlag)
	require.Equal(t, "mpv", cli.SavePlayers)

	cli, err = ParseArgs([]string{"playerbm", "-s", "mpv"})
	require.NoError(t, err)
	require.True(t, cli.SaveFlag)
	require.Equal(t, "mpv", cli.SavePlayers)

	cli, err = ParseArgs([]string{"playerbm", "--save=mpv"})
	require.NoError(t, err)
	require.True(t, cli.SaveFlag)
	require.Equal(t, "mpv", cli.SavePlayers)

	cli, err = ParseArgs([]string{"playerbm", "-s=mpv"})
	require.NoError(t, err)
	require.True(t, cli.SaveFlag)
	require.Equal(t, "mpv", cli.SavePlayers)

	cli, err = ParseArgs([]string{"playerbm", "--resume", "~/file.mp3"})
	require.NoError(t, err)
	require.True(t, cli.ResumeFlag)
	require.Equal(t, "file://~/file.mp3", cli.ResumeFile.String())

	cli, err = ParseArgs([]string{"playerbm", "-r", "~/file.mp3"})
	require.NoError(t, err)
	require.True(t, cli.ResumeFlag)
	require.Equal(t, "file://~/file.mp3", cli.ResumeFile.String())
}

// TODO
// func TestCliBadPath(t *testing.T) {}
