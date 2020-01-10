package main

import (
	"database/sql"
	"fmt"
	"github.com/altdesktop/playerbm/internal/cli"
	"github.com/altdesktop/playerbm/internal/model"
	"github.com/altdesktop/playerbm/internal/player"
	"github.com/godbus/dbus/v5"
	"github.com/hashicorp/logutils"
	"github.com/kyoh86/xdg"
	"log"
	"os"
	"os/exec"
	"path"
	"regexp"
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
		quoted, err := b.Url.ShellQuoted()
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
		if len(b.Hash) >= 7 {
			fmt.Printf(b.Hash[:7] + "  ")
		} else {
			fmt.Printf("%s", "         ")
		}
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
		if cliErr, ok := err.(*cli.CliError); ok {
			fmt.Printf("playerbm: %s\n", cliErr.Error())
			os.Exit(1)
		}
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

	bus, err := dbus.SessionBus()
	if err != nil {
		log.Fatal(err)
	}

	if args.DeleteFlag {
		bookmarks, err := model.ListBookmarks(db)
		if err != nil {
			log.Fatal(err)
		}

		var found bool
		for _, bm := range bookmarks {
			if bm.Url.String() == args.DeleteUrl.String() {
				fmt.Printf("playerbm: deleting bookmark\n")
				err = bm.Delete(db)
				if err != nil {
					log.Fatal(err)
				}
				found = true
			}
		}

		if !found {
			fmt.Printf("playerbm: no bookmark found for url: %s\n", args.DeleteUrl.String())
			os.Exit(1)
		}

		os.Exit(0)
	}

	if args.ResumeFlag {
		var err error

		xdgOpen, err := exec.LookPath("xdg-open")
		if err != nil {
			fmt.Printf("playerbm: resuming requires xdg-open to be in the PATH (provided by xdg-utils)\n")
			os.Exit(127)
		}

		if args.ResumeUrl == nil {
			bookmark, err := model.GetMostRecentBookmark(db)
			if err != nil {
				log.Fatal(err)
			}
			if bookmark == nil {
				fmt.Fprintf(os.Stderr, "No recent unfinished bookmarks found\n")
				os.Exit(0)
			}
			args.ResumeUrl = bookmark.Url
		}

		names, err := player.ListPlayers(bus)
		if err != nil {
			log.Fatal(err)
		}

		for _, name := range names {
			p := player.New(args, db, bus)
			p.SetName(name)
			properties, err := p.GetPropertiesRemote()
			if err != nil {
				log.Printf("[WARNING] could not get properties for player: %s", name)
				continue
			}
			p.SetPlayerProperties(properties)
			if properties.Url != nil && (properties.Url.String() == args.ResumeUrl.String()) {
				log.Printf("[DEBUG] found a running player")
				err = p.LoadBookmark(args.ResumeUrl)
				if err != nil {
					log.Printf("[WARNING] could not load bookmark for player: %s", name)
					continue
				}
				err = p.Manage()
				if err != nil {
					fmt.Printf("playerbm: could not manage player: %s\n", err.Error())
					os.Exit(1)
				}
				os.Exit(p.ExitCode)
			}
		}

		quoted, err := args.ResumeUrl.ShellQuoted()
		if err != nil {
			log.Fatal(err)
		}
		args.PlayerCmd = fmt.Sprintf("%s %s", xdgOpen, quoted)
	}

	if args.ListPlayersFlag {
		bus, err := dbus.SessionBus()
		if err != nil {
			log.Fatal(err)
		}

		names, err := player.ListPlayers(bus)
		if err != nil {
			log.Fatal(err)
		}

		for _, name := range names {
			fmt.Printf("%s\n", name)
		}

		os.Exit(0)
	}

	if args.SaveFlag {
		var err error
		var names []string

		if len(args.SavePlayers) > 0 {
			names = strings.Split(args.SavePlayers, ",")
			for i, name := range names {
				names[i] = strings.Trim(name, " ")
			}
		} else {
			names, err = player.ListPlayers(bus)
			if err != nil {
				log.Fatal(err)
			}
		}

		if len(names) == 0 {
			fmt.Printf("playerbm: no players were found\n")
			os.Exit(1)
		}

		IsNameValid := regexp.MustCompile(`^[0-9a-zA-Z._-]+$`).MatchString

		for _, name := range names {
			if !IsNameValid(name) {
				fmt.Printf("playerbm: got invalid player name: %s\n", name)
				os.Exit(1)
			}
		}

		for _, name := range names {
			p := player.New(args, db, bus)
			p.SetName(name)
			err = p.EnsureBookmark()
			if err != nil {
				fmt.Printf("playerbm: could not save bookmark for player %s: %s\n", name, err.Error())
				continue
			}

			p.Bookmark.Position = p.Position
			err = p.SaveBookmark()
			if err != nil {
				log.Fatal(err)
			}

			fmt.Printf("playerbm: saved bookmark for player %s to position %s\n", name, player.FormatPosition(p.Bookmark.Position))
		}

		os.Exit(0)
	}

	p := player.New(args, db, bus)
	err = p.RunCmd()
	if err != nil {
		if err, ok := err.(*player.PlayerCmdError); ok {
			fmt.Printf("playerbm: %s\n", err.Error())
			os.Exit(err.ExitCode)
		}

		log.Fatal(err)
	}

	os.Exit(p.ExitCode)
}
