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
	"path"
	"strconv"
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

	// get the longest url
	maxUrlLen := 0
	for _, b := range bookmarks {
		l := len(b.Url)
		if l > maxUrlLen {
			maxUrlLen = l
		}
	}

	urlFormat := "%-" + strconv.Itoa(maxUrlLen+2) + "v"

	fmt.Fprintf(os.Stderr, urlFormat, "URL")
	fmt.Fprintf(os.Stderr, "%-18v", "SHA256")
	fmt.Fprintf(os.Stderr, "POSITION")
	fmt.Fprintf(os.Stderr, "\n")

	for _, b := range bookmarks {
		fmt.Printf(urlFormat, b.Url)
		fmt.Printf(b.Hash[:16] + "  ")
		fmt.Printf(player.FormatPosition(b.Position))
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

	if args.ListBookmarksFlag {
		err = handleListBookmarks(db)
		if err != nil {
			log.Fatal(err)
		}
		os.Exit(0)
	}

	p, err := player.InitPlayer(args, db)
	if err != nil {
		log.Fatal(err)
	}

	err = p.Run()
	if err != nil {
		log.Fatal(err)
	}
}
