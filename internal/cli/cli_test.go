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

	cli, err = ParseArgs([]string{"playerbm", "--resume", "/file.mp3"})
	require.NoError(t, err)
	require.True(t, cli.ResumeFlag)
	require.Equal(t, "file:///file.mp3", cli.ResumeUrl.String())

	cli, err = ParseArgs([]string{"playerbm", "-r", "/file.mp3"})
	require.NoError(t, err)
	require.True(t, cli.ResumeFlag)
	require.Equal(t, "file:///file.mp3", cli.ResumeUrl.String())

	cli, err = ParseArgs([]string{"playerbm", "-d", "/file.mp3"})
	require.NoError(t, err)
	require.True(t, cli.DeleteFlag)
	require.Equal(t, "file:///file.mp3", cli.DeleteUrl.String())

}

func TestFileWithSpaces(t *testing.T) {
	cli, err := ParseArgs([]string{"playerbm", "-d", "/file with spaces.mp3"})
	require.NoError(t, err)
	require.Equal(t, "file:///file%20with%20spaces.mp3", cli.DeleteUrl.String())
	quoted := cli.DeleteUrl.ShellQuoted()
	require.Equal(t, quoted, "'/file with spaces.mp3'")

	cli, err = ParseArgs([]string{"playerbm", "-d", "/Comedy - Ep.#3 A Secret Society (w_ Jason Ritter, Craig Cackowski, Amanda Lund, Chris Tallman)-9HuAXgbdFx4.opus"})
	require.NoError(t, err)
	require.Equal(t, "file", cli.DeleteUrl.Scheme())
	quoted = cli.DeleteUrl.ShellQuoted()
	require.NoError(t, err)
	require.Equal(t, "'/Comedy - Ep.#3 A Secret Society (w_ Jason Ritter, Craig Cackowski, Amanda Lund, Chris Tallman)-9HuAXgbdFx4.opus'", quoted)
	require.Equal(t, "/Comedy - Ep.#3 A Secret Society (w_ Jason Ritter, Craig Cackowski, Amanda Lund, Chris Tallman)-9HuAXgbdFx4.opus", cli.DeleteUrl.UnescapedPath())
}

// TODO
// func TestCliBadPath(t *testing.T) {}
