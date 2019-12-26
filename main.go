package main

import (
	"database/sql"
	"fmt"
	"github.com/altdesktop/playerbm/cmd/cli"
	"github.com/altdesktop/playerbm/cmd/model"
	"github.com/altdesktop/playerbm/cmd/player"
	"log"
	"os"
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

func main() {
	cli, err := cli.ParseArgs(os.Args)
	if err != nil {
		log.Fatal(err)
	}

	db, err := model.InitDb(cli.DBPath)
	if err != nil {
		log.Fatal(err)
	}

	if cli.ListBookmarksFlag {
		err = handleListBookmarks(db)
		if err != nil {
			log.Fatal(err)
		}
		os.Exit(0)
	}

	p, err := player.InitPlayer(cli, db)
	if err != nil {
		log.Fatal(err)
	}

	err = p.Run()
	if err != nil {
		log.Fatal(err)
	}
}
