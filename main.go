package main

import (
	"database/sql"
	"fmt"
	"github.com/altdesktop/playerbm/cmd/cli"
	"github.com/altdesktop/playerbm/cmd/model"
	"github.com/altdesktop/playerbm/cmd/player"
	"github.com/hashicorp/logutils"
	"github.com/kyoh86/xdg"
	"log"
	"os"
	"os/exec"
	"path"
	"strconv"
	"strings"
)

func handleListBookmarks(db *sql.DB) error {
	bookmarks, err := model.ListBookmarks(db)
	if err != nil {
		return err
	}

	if len(bookmarks) == 0 {
		// nothing to do
		return nil
	}

	urls := []string{}

	home := os.Getenv("HOME")

	// get the longest url
	maxUrlLen := 0
	for _, b := range bookmarks {
		// TODO update me for http scheme
		quoted, err := model.UrlShellQuoted(b.Url)
		if err != nil {
			log.Fatal(err)
		}

		// this is nice for me
		if home != "" && strings.HasPrefix(quoted, home) {
			quoted = strings.Replace(quoted, home, "~", 1)
		}

		l := len(quoted)
		if l > maxUrlLen {
			maxUrlLen = l
		}
		urls = append(urls, quoted)
	}

	urlFormat := "%-" + strconv.Itoa(maxUrlLen+2) + "v"

	fmt.Fprintf(os.Stderr, urlFormat, "URL")
	fmt.Fprintf(os.Stderr, "%-9v", "SHA256")
	fmt.Fprintf(os.Stderr, "POSITION")
	fmt.Fprintf(os.Stderr, "\n")

	for i, b := range bookmarks {
		fmt.Printf(urlFormat, urls[i])
		fmt.Printf(b.Hash[:7] + "  ")
		positionFormatted := player.FormatPosition(b.Position)
		if b.Length > 0 {
			positionFormatted = positionFormatted + "/" + player.FormatPosition(b.Length)
		}
		fmt.Printf(positionFormatted)
		fmt.Printf("\n")
	}

	return nil
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

func setupDBPath() (string, error) {
	pathDir := path.Join(xdg.CacheHome(), "playerbm")
	err := os.MkdirAll(pathDir, 0775)
	if err != nil {
		return "", err
	}
	return path.Join(pathDir, "bookmarks.db"), nil
}

func main() {
	setupLogging()

	args, err := cli.ParseArgs(os.Args)
	if err != nil {
		log.Fatal(err)
	}

	if args.HelpFlag {
		fmt.Print(cli.HelpString)
		os.Exit(0)
	}

	if args.VersionFlag {
		fmt.Print(cli.VersionString)
		os.Exit(0)
	}

	dbPath, err := setupDBPath()
	if err != nil {
		log.Fatal(err)
	}

	db, err := model.InitDb(dbPath)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	if args.ListBookmarksFlag {
		err = handleListBookmarks(db)
		if err != nil {
			log.Fatal(err)
		}
		os.Exit(0)
	}

	if args.ResumeFlag {
		xdgOpen, err := exec.LookPath("xdg-open")
		if err != nil {
			fmt.Printf("Resuming requires xdg-open to be in the PATH (provided by xdg-utils)\n")
			os.Exit(127)
		}

		recentUrl, err := model.GetMostRecentUrl(db)
		if err != nil {
			log.Fatal(err)
		}
		if recentUrl == nil {
			fmt.Fprintf(os.Stderr, "No recent unfinished bookmarks found\n")
			os.Exit(0)
		}
		quoted, err := model.UrlShellQuoted(recentUrl)
		if err != nil {
			log.Fatal(err)
		}
		args.PlayerCmd = fmt.Sprintf("%s %s", xdgOpen, quoted)
	}

	p := player.New(args, db)

	if args.ListPlayersFlag {
		names, err := p.ListPlayers()
		if err != nil {
			log.Fatal(err)
		}

		for _, name := range names {
			fmt.Printf("%s\n", name)
		}

		os.Exit(0)
	}

	err = p.Run()
	if err != nil {
		if err, ok := err.(*player.PlayerCmdError); ok {
			fmt.Printf("playerbm: %s\n", err.Error())
			os.Exit(err.ExitCode)
		}

		log.Fatal(err)
	}
}
