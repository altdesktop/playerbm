package player

import (
	"database/sql"
	"errors"
	"fmt"
	"github.com/altdesktop/playerbm/cmd/cli"
	"github.com/altdesktop/playerbm/cmd/model"
	"github.com/godbus/dbus/v5"
	"github.com/mitchellh/go-ps"
	"log"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

const (
	Playing = "Playing"
	Paused  = "Paused"
	Stopped = "Stopped"
)

type Player struct {
	DB              *sql.DB
	Bus             *dbus.Conn
	Cli             *cli.PbmCli
	Cmd             *exec.Cmd
	CurrentBookmark *model.Bookmark
	BusName         string
	NameOwner       string
	MprisObj        dbus.BusObject
	Position        int64
	PositionTime    time.Time
	TrackId         dbus.ObjectPath
	HasTrackId      bool
	Status          string
	Length          int64
	ProcessFinish   chan error
}

type PlayerCmdError struct {
	err      string
	ExitCode int
}

func (e *PlayerCmdError) Error() string {
	return e.err
}

func isChildProcess(p int, child int) (bool, error) {
	if p == child {
		return true, nil
	}
	childProcess, err := ps.FindProcess(child)
	if err != nil {
		return false, err
	}
	if childProcess == nil {
		return false, nil
	}
	if childProcess.PPid() == p {
		return true, nil
	} else {
		return isChildProcess(p, childProcess.PPid())
	}
}

func (player *Player) initDbus() error {
	if player.Bus != nil {
		return nil
	}

	bus, err := dbus.SessionBus()
	if err != nil {
		return err
	}

	player.Bus = bus
	return nil
}

func New(cli *cli.PbmCli, db *sql.DB) *Player {
	return &Player{
		Cli:           cli,
		DB:            db,
		Status:        Stopped,
		ProcessFinish: make(chan error),
	}
}

func (player *Player) ListPlayers() ([]string, error) {
	prefix := "org.mpris.MediaPlayer2."
	players := []string{}
	err := player.initDbus()

	if err != nil {
		return players, err
	}
	names := []string{}
	err = player.Bus.BusObject().Call("org.freedesktop.DBus.ListNames", 0).Store(&names)
	if err != nil {
		return players, err
	}

	for _, name := range names {
		if strings.HasPrefix(name, prefix) {
			players = append(players, name[len(prefix):])
		}
	}

	return players, nil
}

func (player *Player) getProperties() (map[string]dbus.Variant, error) {
	var properties map[string]dbus.Variant
	err := player.MprisObj.Call("org.freedesktop.DBus.Properties.GetAll", 0, "org.mpris.MediaPlayer2.Player").Store(&properties)
	if err != nil {
		return nil, err
	}
	return properties, nil
}

func (player *Player) setProperties(properties map[string]dbus.Variant) {
	var position int64
	var hasPosition bool
	var length int64
	var hasLength bool
	var url *url.URL
	var status string
	var hasStatus bool
	var queueUpdate bool

	// get the position
	if variant, found := properties["Position"]; found {
		if val, ok := variant.Value().(int64); ok {
			position = val
			hasPosition = true
		}
	}

	if metadataVariant, found := properties["Metadata"]; found {
		if metadata, ok := metadataVariant.Value().(map[string]dbus.Variant); ok {
			// get the length
			if variant, ok := metadata["mpris:length"]; ok {
				if val, ok := variant.Value().(int64); ok {
					hasLength = true
					length = val
				}
			}

			// get the trackid
			if variant, found := metadata["mpris:trackid"]; found {
				if val, ok := variant.Value().(dbus.ObjectPath); ok {
					player.TrackId = val
				}
			}

			// get the url
			if variant, found := metadata["xesam:url"]; found {
				if val, ok := variant.Value().(string); ok {
					normalizedUrl, err := model.ParseXesamUrl(val)
					if err != nil {
						log.Printf("[DEBUG] player gave invalid url: %s", val)
					} else {
						url = normalizedUrl
					}
				}
			}
		}
	}

	// get the playback status
	if variant, found := properties["PlaybackStatus"]; found {
		if val, ok := variant.Value().(string); ok {
			hasStatus = true
			status = val
		}
	}

	currentUrl := player.currentUrl()
	if url != nil && (currentUrl == nil || url.String() != currentUrl.String()) {
		log.Printf("[DEBUG] url has changed from '%s' to '%s'", currentUrl, url)
		err := player.updateBookmark()
		if err != nil {
			log.Printf("[DEBUG] could not update current bookmark: %+v", err)
		}
		err = player.loadBookmark(url)
		if err != nil {
			log.Printf("[DEBUG] could not load bookmark: %+v", err)
		}
		queueUpdate = true
	}

	if hasLength && player.CurrentBookmark != nil && player.CurrentBookmark.Length != length {
		log.Printf("[DEBUG] setting player length to '%s'", FormatPosition(length))
		player.CurrentBookmark.Length = length
	}

	if hasStatus && status != player.Status {
		log.Printf("[DEBUG] playback status has changed from '%s' to '%s'", player.Status, status)
		switch status {
		case Playing:
			player.PositionTime = time.Now()
		case Paused:
		case Stopped:
			// TODO: no track currently playing if stopped
			player.Position = player.currentPosition()
		default:
			log.Printf("[DEBUG] player gave invalid status: %s", status)
		}
		queueUpdate = true
		player.Status = status
	}

	if hasPosition {
		log.Printf("[DEBUG] position has changed from '%s' to '%s'", FormatPosition(player.currentPosition()), FormatPosition(position))
		player.setPosition(position)
	}

	player.logPosition()
	player.logCurrentBookmark()

	if queueUpdate {
		// Run this if anything important has changed. This works around spec
		// weirdness regarding position.
		go func() {
			properties, err := player.getProperties()
			if err != nil {
				log.Printf("[DEBUG] could not get properties: %+v", err)
				return
			}

			player.setProperties(properties)
		}()
	}
}

func (player *Player) currentUrl() *url.URL {
	if player.CurrentBookmark == nil {
		return nil
	}

	return player.CurrentBookmark.Url
}

func (player *Player) currentPosition() int64 {
	if player.Status == Playing {
		return player.Position + time.Since(player.PositionTime).Microseconds()
	} else {
		return player.Position
	}
}

func FormatPosition(ms int64) string {
	seconds := (ms / 1000000) % 60
	minutes := (ms / 1000000 / 60) % 60
	hours := (ms / 1000000 / 60 / 60)

	if hours > 0 {
		return fmt.Sprintf("%d:%02d:%02d", hours, minutes, seconds)
	} else {
		return fmt.Sprintf("%d:%02d", minutes, seconds)
	}
}

func (player *Player) logPosition() {
	log.Printf("[DEBUG] current position: %s", FormatPosition(player.currentPosition()))
}

func (player *Player) logCurrentBookmark() {
	if player.CurrentBookmark == nil {
		log.Printf("[DEBUG] no current bookmark")
	} else {
		log.Printf("[DEBUG] bookmark: url=%s, position=%s", player.CurrentBookmark.Url, FormatPosition(player.CurrentBookmark.Position))
	}
}

func (player *Player) setPosition(ms int64) {
	player.Position = ms
	player.PositionTime = time.Now()
}

func (player *Player) syncPosition(ms int64) error {
	if len(player.TrackId) == 0 {
		return errors.New("Player does not have a trackid")
	}

	if !player.TrackId.IsValid() {
		return errors.New(fmt.Sprintf("Player has an invalid trackid: '%s'", player.TrackId))
	}

	log.Printf("[DEBUG] Syncing player position to %s", FormatPosition(ms))
	err := player.MprisObj.Call("org.mpris.MediaPlayer2.Player.Play", 0).Store()
	if err != nil {
		return err
	}
	err = player.MprisObj.Call("org.mpris.MediaPlayer2.Player.SetPosition", 0, player.TrackId, ms).Store()
	if err != nil {
		return err
	}
	player.setPosition(ms)
	return nil
}

func (player *Player) handleSeeked(message *dbus.Signal) {
	log.Printf("[DEBUG] handling seeked: %+v", message)
	if seeked, ok := message.Body[0].(int64); ok {
		player.setPosition(seeked)
	} else {
		log.Printf("[DEBUG] got invalid seeked value")
	}

	player.logPosition()
}

func (player *Player) handlePropertiesChanged(message *dbus.Signal) {
	log.Printf("[DEBUG] handling properties changed: %+v", message)
	name := fmt.Sprintf("%s", message.Body[0])

	if name != "org.mpris.MediaPlayer2.Player" {
		return
	}

	if properties, ok := message.Body[1].(map[string]dbus.Variant); ok {
		player.setProperties(properties)
	}
}

func (player *Player) handleNameOwnerChanged(message *dbus.Signal) bool {
	name := fmt.Sprintf("%s", message.Body[0])
	// oldOwner := fmt.Sprintf("%s", message.Body[1])
	newOwner := fmt.Sprintf("%s", message.Body[2])

	if name != player.BusName {
		return false
	}

	log.Printf("[DEBUG] handling name owner changed: %+v", message)

	if newOwner != player.NameOwner {
		// XXX: the name could actually be transferred, but I've never seen
		// this.
		log.Printf("[DEBUG] name owner changed from '%s' to '%s'", player.NameOwner, newOwner)
		return true
	}

	return false
}

func (player *Player) loadBookmark(url *url.URL) error {
	player.logCurrentBookmark()

	if player.CurrentBookmark != nil && player.CurrentBookmark.Url == url {
		log.Printf("[DEBUG] url unchanged, not loading bookmark")
		return nil
	}

	bookmark, err := model.GetBookmark(player.DB, url)
	if err != nil {
		return err
	}

	if bookmark.Exists() {
		log.Printf("[DEBUG] bookmark exists, syncing to position %s", FormatPosition(bookmark.Position))
		time.Sleep(100 * time.Millisecond)
		err = player.syncPosition(bookmark.Position)
		if err != nil {
			log.Printf("[DEBUG] could not sync position: %+v", err)
		}
	} else {
		log.Printf("[DEBUG] bookmark does not exist, not restoring")
	}

	player.CurrentBookmark = bookmark
	player.logCurrentBookmark()

	return nil
}

func (player *Player) updateBookmark() error {
	if player.CurrentBookmark == nil {
		log.Printf("[DEBUG] no current bookmark to update")
		return nil
	}

	position := player.currentPosition()
	log.Printf("[DEBUG] saving bookmark to position: %s", FormatPosition(position))
	player.CurrentBookmark.Position = position
	player.logCurrentBookmark()
	return player.CurrentBookmark.Save(player.DB)
}

func (player *Player) installSignalHandlers() {
	go func() {
		signals := make(chan os.Signal, 10)
		signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
		for {
			s := <-signals
			err := player.Cmd.Process.Signal(s)
			if err != nil {
				log.Printf("[WARNING] could not send signal to player process: %+v", err)
			}
		}
	}()
}

func (player *Player) init() error {
	err := player.initDbus()
	if err != nil {
		return err
	}

	busObj := player.Bus.BusObject()

	err = player.Bus.AddMatchSignal(
		dbus.WithMatchSender("org.freedesktop.DBus"),
		dbus.WithMatchInterface("org.freedesktop.DBus"),
		dbus.WithMatchObjectPath("/org/freedesktop/DBus"),
		dbus.WithMatchMember("NameOwnerChanged"),
	)
	if err != nil {
		return err
	}

	c := make(chan *dbus.Signal, 10)
	player.Bus.Signal(c)
	defer player.Bus.RemoveSignal(c)

	log.Printf("[DEBUG] %s", player.Cli.PlayerCmd)
	// TODO: if the process exits nonzero, break the main loop
	player.Cmd = exec.Command("/bin/bash", "-c", player.Cli.PlayerCmd)
	player.Cmd.Stdout = os.Stdout
	player.Cmd.Stderr = os.Stderr

	go func() {
		err = player.Cmd.Run()
		c <- nil
		player.ProcessFinish <- err
	}()

	for message := range c {
		if message == nil {
			break
		}
		name := fmt.Sprintf("%s", message.Body[0])
		// oldOwner := fmt.Sprintf("%s", message.Body[1])
		newOwner := fmt.Sprintf("%s", message.Body[2])

		if !strings.HasPrefix(name, "org.mpris.MediaPlayer2.") {
			continue
		}

		if len(newOwner) > 0 {
			log.Printf("[DEBUG] a player appeared: name: %s, owner: %s", name, newOwner)
			var pid int
			err := busObj.Call("org.freedesktop.DBus.GetConnectionUnixProcessID", 0, name).Store(&pid)
			if err != nil {
				log.Printf("[DEBUG] could not get process id: %+v", err)
				continue
			}
			log.Printf("[DEBUG] pid: %d", pid)
			processMatch, err := isChildProcess(player.Cmd.Process.Pid, pid)
			if err != nil {
				log.Printf("[DEBUG] could not get process info: %+v", err)
				continue
			}
			// TODO handle the case where no process is opened directly, but
			// the file is opened through ipc to an existing process. Firefox
			// does this.
			if processMatch {
				log.Printf("[DEBUG] managing player")
				player.BusName = name
				player.NameOwner = newOwner
				player.MprisObj = player.Bus.Object(name, "/org/mpris/MediaPlayer2")
				break
			}
		}
	}

	if player.MprisObj == nil {
		if player.Cmd.ProcessState.Exited() {
			exitCode := player.Cmd.ProcessState.ExitCode()
			err = &PlayerCmdError{
				err:      fmt.Sprintf("player process exited unexpectedly (exit %d)", exitCode),
				ExitCode: exitCode,
			}
			return err
		} else {
			panic("should not be reached (TODO: implement the dbus timeout)")
		}
	}

	return nil
}

func (player *Player) Run() error {
	err := player.init()
	if err != nil {
		return err
	}
	properties, err := player.getProperties()
	if err != nil {
		return err
	}

	player.setProperties(properties)
	player.installSignalHandlers()

	defer func() {
		err = player.updateBookmark()
		if err != nil {
			log.Printf("[DEBUG] could not update bookmark: %+v", err)
		}
	}()

	// start the main loop
	c := make(chan *dbus.Signal, 10)
	player.Bus.Signal(c)
	defer player.Bus.RemoveSignal(c)

	err = player.Bus.AddMatchSignal(
		dbus.WithMatchSender(player.NameOwner),
		dbus.WithMatchObjectPath("/org/mpris/MediaPlayer2"),
	)
	if err != nil {
		return err
	}

	for message := range c {
		if message.Sender == player.NameOwner && message.Path == "/org/mpris/MediaPlayer2" {
			if message.Name == "org.mpris.MediaPlayer2.Player.Seeked" {
				player.handleSeeked(message)
			} else if message.Name == "org.freedesktop.DBus.Properties.PropertiesChanged" {
				iface := fmt.Sprintf("%s", message.Body[0])
				if iface == "org.mpris.MediaPlayer2.Player" {
					player.handlePropertiesChanged(message)
				}
			}
		} else if message.Name == "org.freedesktop.DBus.NameOwnerChanged" && message.Sender == "org.freedesktop.DBus" {
			if player.handleNameOwnerChanged(message) {
				log.Printf("[DEBUG] name lost, shutting down")
				break
			}
		}
	}

	return <-player.ProcessFinish
}
