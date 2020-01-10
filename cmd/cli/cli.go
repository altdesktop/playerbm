package cli

import (
	"fmt"
	"github.com/altdesktop/playerbm/cmd/model"
	"github.com/kballard/go-shellquote"
	"log"
	"strings"
)

type PbmCli struct {
	PlayerCmd         string
	ListBookmarksFlag bool
	ListPlayersFlag   bool
	HelpFlag          bool
	VersionFlag       bool
	ResumeFlag        bool
	ResumeUrl         *model.XesamUrl
	SaveFlag          bool
	SavePlayers       string
	DeleteFlag        bool
	DeleteUrl         *model.XesamUrl
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
   -l, --list-bookmarks  List all bookmarks and exit.
   -L, --list-players    List all running players that can be controlled.
   -r, --resume=[URL]    Launch a player and resume playing URL from the last
                         saved bookmark and begin managing bookmarks. (default:
                         file of the last saved bookmark)
   -s, --save=[PLAYER]   Save bookmarks for the running players in a comma
                         separated list. (default: all running players)
   -d, --delete={URL}    Delete the bookmark for the given url.
   -h, --help            Show help.
   -v, --version         Print the version.` + "\n"

const VersionString = "v0.0.1\n"

type BoolFlag struct {
	Short string
	Long  string
	Value *bool
}

type StringFlag struct {
	Short           string
	Long            string
	Present         *bool
	ArgValuePresent bool
	ArgValue        *string
}

type CliError struct {
	err string
}

func (e *CliError) Error() string {
	return e.err
}

func newCliError(format string, args ...interface{}) *CliError {
	return &CliError{err: fmt.Sprintf(format, args...)}
}

func parseBoolFlag(arg string, flag BoolFlag) (bool, error) {
	if arg == flag.Short || arg == flag.Long {
		*flag.Value = true
		return true, nil
	}

	if strings.HasPrefix(arg, flag.Short+"=") || strings.HasPrefix(arg, flag.Long+"=") {
		return false, newCliError("no argument expected for %s, %s flag", flag.Short, flag.Long)
	}

	return false, nil
}

func parseStringFlag(arg string, flag StringFlag) (bool, bool, error) {
	if arg == flag.Short || arg == flag.Long {
		*flag.Present = true
		return true, false, nil
	}

	if strings.HasPrefix(arg, flag.Short+"=") || strings.HasPrefix(arg, flag.Long+"=") {
		*flag.Present = true
		*flag.ArgValue = strings.Split(arg, "=")[1]
		return true, true, nil
	}

	return false, false, nil
}

func ParseArgs(args []string) (*PbmCli, error) {
	var err error

	log.Printf("[DEBUG] parsing arguments: %v", args)

	cli := PbmCli{}

	if len(args) == 1 {
		cli.HelpFlag = true
		return &cli, nil
	}

	boolFlags := []BoolFlag{
		BoolFlag{Short: "-v", Long: "--version", Value: &cli.VersionFlag},
		BoolFlag{Short: "-h", Long: "--help", Value: &cli.HelpFlag},
		BoolFlag{Short: "-l", Long: "--list-bookmarks", Value: &cli.ListBookmarksFlag},
		BoolFlag{Short: "-L", Long: "--list-players", Value: &cli.ListPlayersFlag},
	}

	var resumeUrl string
	var deleteUrl string
	stringFlags := []StringFlag{
		StringFlag{Short: "-s", Long: "--save", Present: &cli.SaveFlag, ArgValue: &cli.SavePlayers},
		StringFlag{Short: "-r", Long: "--resume", Present: &cli.ResumeFlag, ArgValue: &resumeUrl},
		StringFlag{Short: "-d", Long: "--delete", Present: &cli.DeleteFlag, ArgValue: &deleteUrl},
	}

	firstPlayerArg := -1
	skipNext := true
	for i, arg := range args {
		if skipNext {
			skipNext = false
			continue
		}

		if strings.HasPrefix(arg, "-") {
			var match bool

			for _, flag := range boolFlags {
				match, err = parseBoolFlag(arg, flag)
				if err != nil {
					return nil, err
				}

				if match {
					break
				}
			}

			if !match {
				for _, flag := range stringFlags {
					var argValuePresent bool
					match, argValuePresent, err = parseStringFlag(arg, flag)
					if err != nil {
						return nil, err
					}
					if match {
						if !argValuePresent {
							// look ahead
							if len(args) < i+2 || strings.HasPrefix(args[i+1], "-") {
								break
							}
							*flag.ArgValue = args[i+1]
							skipNext = true
						}
						break
					}
				}
			}

			if !match {
				return nil, newCliError("unknown argument: %s", arg)
			}
		} else {
			firstPlayerArg = i
			break
		}
	}

	if cli.ResumeFlag && len(resumeUrl) > 0 {
		cli.ResumeUrl, err = model.ParseXesamUrl(resumeUrl)
		if err != nil {
			return nil, newCliError("could not parse url: %s", resumeUrl)
		}
	}

	if cli.DeleteFlag {
		if len(deleteUrl) == 0 {
			return nil, newCliError("a URL argument is required for the delete flag")
		}
		cli.DeleteUrl, err = model.ParseXesamUrl(deleteUrl)
		if err != nil {
			return nil, newCliError("could not parse url: %s", deleteUrl)
		}
	}

	if firstPlayerArg != -1 {
		cli.PlayerCmd = shellquote.Join(args[firstPlayerArg:]...)
	}

	// TODO: argument validation

	log.Printf("[DEBUG] args: %+v", cli)

	return &cli, nil
}
