package cli

import (
	"github.com/hashicorp/logutils"
	"github.com/kyoh86/xdg"
	"github.com/urfave/cli/v2"
	"io"
	"log"
	"os"
	"path"
	"strings"
)

type PbmCli struct {
	PlayerCmd         string
	DBPath            string
	app               cli.App
	ListBookmarksFlag bool
}

func setupDBPath() (string, error) {
	pathDir := path.Join(xdg.CacheHome(), "playerbm")
	// TODO move directory making to InitDB()
	err := os.MkdirAll(pathDir, 0775)
	if err != nil {
		return "", err
	}
	return path.Join(pathDir, "bookmarks.db"), nil
}

func setupLogging() {
	loglevel := os.Getenv("PBM_LOGLEVEL")
	if loglevel == "" {
		loglevel = "WARN"
	}
	filter := &logutils.LevelFilter{
		Levels:   []logutils.LogLevel{"DEBUG", "WARN", "ERROR"},
		MinLevel: logutils.LogLevel(loglevel),
		Writer:   os.Stderr,
	}
	log.SetOutput(filter)
}

func partitionArgs(args []string) ([]string, string) {
	if len(args) == 1 {
		return args, ""
	}

	pbmArgs := []string{}

	firstPlayerArg := -1
	for i, arg := range args {
		if i == 0 {
			pbmArgs = append(pbmArgs, arg)
			continue
		}

		if strings.HasPrefix(arg, "-") || strings.HasPrefix(arg, "--") {
			pbmArgs = append(pbmArgs, arg)
			continue
		} else {
			firstPlayerArg = i
			break
		}
	}

	playerCmd := ""
	if firstPlayerArg != -1 {
		playerCmd = strings.Join(args[firstPlayerArg:], " ")
	}

	return pbmArgs, playerCmd
}

func ParseArgs(args []string) (*PbmCli, error) {
	setupLogging()

	log.Printf("[DEBUG] parsing arguments: %v", args)

	dbpath, err := setupDBPath()
	if err != nil {
		return nil, err
	}

	listBookmarksFlag := false

	// TODO fix this bug in the cli package
	oldHelpPrinter := cli.HelpPrinter
	cli.HelpPrinter = func(w io.Writer, templ string, data interface{}) {
		// TODO we should pass this up and let main do the printing and exiting
		oldHelpPrinter(w, templ, data)
		os.Exit(0)
	}

	oldVersionPrinter := cli.VersionPrinter
	cli.VersionPrinter = func(c *cli.Context) {
		// TODO we should pass this up and let main do the printing and exiting
		oldVersionPrinter(c)
		os.Exit(0)
	}

	app := &cli.App{
		Version: "0.0.1",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:    "list-bookmarks",
				Value:   false,
				Aliases: []string{"l"},
				Usage:   "list all bookmarks and exit",
			},
		},
		Action: func(c *cli.Context) error {
			listBookmarksFlag = c.Bool("list-bookmarks")
			return nil
		},
	}
	app.CustomAppHelpTemplate = `Usage:
    {{.Name}} [OPTION…] PLAYER_COMMAND

Description:
    {{.Name}} is a utility that saves your place when you exit the player or
    change the track and automatically resumes from where you left off when you
    open it again.

    Pass the command to open your media player as PLAYER_COMMAND and {{.Name}}
    will connect to the player over the MPRIS DBus Specification and begin
    managing bookmarks.

Example:
    {{.Name}} player ~/audiobooks/war-and-peace.mp3

    Listen for awhile and close the player. When you open the player again with
    {{.Name}}, it will seek to your last position.

Options:
   {{range $index, $option := .VisibleFlags}}{{if $index}}
   {{end}}{{$option}}{{end}}` + "\n"
	app.Setup()
	app.Commands = []*cli.Command{}

	pbmArgs, playerCmd := partitionArgs(args)

	log.Printf("[DEBUG] pbm args: %v", pbmArgs)
	log.Printf("[DEBUG] player command: '%s'", playerCmd)

	if len(pbmArgs) == 1 && playerCmd == "" {
		pbmArgs = append(pbmArgs, "--help")
	}

	err = app.Run(pbmArgs)
	if err != nil {
		return nil, err
	}

	ctx := PbmCli{
		DBPath:            dbpath,
		PlayerCmd:         playerCmd,
		ListBookmarksFlag: listBookmarksFlag,
	}

	return &ctx, nil
}
