package cli

import (
	"errors"
	"fmt"
	"log"
	"strings"
)

type PbmCli struct {
	PlayerCmd         string
	ListBookmarksFlag bool
	HelpFlag          bool
	VersionFlag       bool
}

const HelpString = `playerbm [OPTION…] PLAYER_COMMAND

Description:
    playerbm is a utility that saves your place when you exit the player or
    change the track and automatically resumes from where you left off when you
    open it again.

    Pass the command to open your media player as PLAYER_COMMAND and playerbm
    will connect to the player over the MPRIS DBus Specification and begin
    managing bookmarks.

Example:
    playerbm player ~/audiobooks/war-and-peace.mp3

    Listen for awhile and close the player. When you open the player again with
    playerbm, it will seek to your last position.

Options:
   --list-bookmarks, -l  list all bookmarks and exit
   --help, -h            show help
   --version, -v         print the version` + "\n"

const VersionString = "v0.0.1\n"

func ParseArgs(args []string) (*PbmCli, error) {
	log.Printf("[DEBUG] parsing arguments: %v", args)

	cli := PbmCli{}

	if len(args) == 1 {
		cli.HelpFlag = true
		return &cli, nil
	}

	firstPlayerArg := -1
	for i, arg := range args {
		if i == 0 {
			continue
		}

		if arg == "-v" || arg == "--version" {
			cli.VersionFlag = true
			break
		} else if arg == "-h" || arg == "--help" {
			cli.HelpFlag = true
			break
		} else if arg == "-l" || arg == "--list-bookmarks" {
			cli.ListBookmarksFlag = true
			break
		} else if strings.HasPrefix(arg, "-") {
			return nil, errors.New(fmt.Sprintf("Unknown argument: %s", arg))
		} else {
			firstPlayerArg = i
			break
		}
	}

	if firstPlayerArg != -1 {
		cli.PlayerCmd = strings.Join(args[firstPlayerArg:], " ")
	}

	return &cli, nil
}
