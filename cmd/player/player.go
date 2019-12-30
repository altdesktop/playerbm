package player

import (
	"database/sql"
	"errors"
	"fmt"
	"github.com/altdesktop/playerbm/cmd/cli"
	"github.com/altdesktop/playerbm/cmd/model"
	"github.com/godbus/dbus/v5"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

type PlaybackStatus string

const (
	Playing PlaybackStatus = "Playing"
	Paused  PlaybackStatus = "Paused"
	Stopped PlaybackStatus = "Stopped"
)

func parsePlaybackStatus(status string) (PlaybackStatus, bool) {
	switch status {
	case "Playing":
		return Playing, true
	case "Paused":
		return Paused, true
	case "Stopped":
		return Stopped, true
	default:
		return "", false
	}
}

type Player struct {
	DB              *sql.DB
	Bus             *dbus.Conn
	Cli             *cli.PbmCli
	Cmd             *exec.Cmd
	CurrentBookmark *model.Bookmark
	BusName         string
	NameOwner       string
	MprisObj        dbus.BusObject
	Url             string
	HasUrl          bool
	Position        int64
	PositionTime    time.Time
	TrackId         dbus.ObjectPath
	HasTrackId      bool
	Status          PlaybackStatus
	Length          int64
}

func abs(x int64) int64 {
	if x < 0 {
		return -x
	}
	return x
}

func InitPlayer(cli *cli.PbmCli, db *sql.DB) (*Player, error) {
	player := Player{
		Cli:    cli,
		DB:     db,
		Status: Stopped,
	}

	bus, err := dbus.SessionBus()
	if err != nil {
		return nil, err
	}
	player.Bus = bus

	busObj := bus.BusObject()

	err = bus.AddMatchSignal(
		dbus.WithMatchSender("org.freedesktop.DBus"),
		dbus.WithMatchInterface("org.freedesktop.DBus"),
		dbus.WithMatchObjectPath("/org/freedesktop/DBus"),
		dbus.WithMatchMember("NameOwnerChanged"),
	)
	if err != nil {
		return nil, err
	}

	c := make(chan *dbus.Signal, 10)
	bus.Signal(c)
	defer bus.RemoveSignal(c)

	log.Printf("[DEBUG] %s", cli.PlayerCmd)
	player.Cmd = exec.Command("/bin/bash", "-c", cli.PlayerCmd)
	player.Cmd.Stdout = os.Stdout
	player.Cmd.Stderr = os.Stderr

	err = player.Cmd.Start()
	if err != nil {
		return nil, err
	}

	for message := range c {
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
				log.Printf("[DEBUG] could not get process id: %v", err)
			}
			log.Printf("[DEBUG] pid: %d", pid)
			if player.Cmd.Process.Pid == pid {
				log.Printf("[DEBUG] managing player")
				player.BusName = name
				player.NameOwner = newOwner
				player.MprisObj = bus.Object(name, "/org/mpris/MediaPlayer2")
				break
			}
		}
	}

	return &player, nil
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
	if metadataVariant, found := properties["Metadata"]; found {
		if metadata, ok := metadataVariant.Value().(map[string]dbus.Variant); ok {
			var hasLength = false
			var length int64

			// get the length
			if variant, ok := metadata["mpris:length"]; ok {
				if val, ok := variant.Value().(int64); ok {
					hasLength = true
					length = val
				}
			}

			// get the trackid
			if variant, found := metadata["mpris:trackid"]; found {
				if trackid, ok := variant.Value().(dbus.ObjectPath); ok {
					player.TrackId = trackid
					if len(trackid) > 0 {
						log.Printf("[DEBUG] trackid found: %s", trackid)
						player.HasTrackId = true
					}
				}
			}

			// get the url
			if variant, found := metadata["xesam:url"]; found {
				if url, ok := variant.Value().(string); ok {
					if len(url) > 0 && player.Url != url {
						log.Printf("[DEBUG] url changed from: '%s' to '%s'", player.Url, url)
						err := player.updateBookmark()
						if err != nil {
							log.Printf("[DEBUG] could not update bookmark: %v", err)
						}
						player.Url = url
						player.HasUrl = true
						if hasLength {
							player.Length = length
						}
						err = player.restoreBookmark()
						if err != nil {
							log.Printf("[DEBUG] could not restore bookmark: %v", err)
						}
					}
				}
			}

			if hasLength {
				player.Length = length
			}
		}
	}

	// get the position
	if variant, found := properties["Position"]; found {
		if position, ok := variant.Value().(int64); ok {
			log.Printf("[DEBUG] position has changed from '%s' to '%s'", FormatPosition(player.currentPosition()), FormatPosition(position))
			player.setPosition(position)
		}
	}

	// get the playback status
	if variant, found := properties["PlaybackStatus"]; found {
		if val, ok := variant.Value().(string); ok {
			if status, ok := parsePlaybackStatus(val); ok {
				if status != player.Status {
					log.Printf("[DEBUG] playback status has changed from '%s' to '%s'", player.Status, status)
					switch status {
					case Playing:
						player.PositionTime = time.Now()
					case Paused:
						player.Position = player.currentPosition()
					case Stopped:
						// TODO: delete the bookmark, probably on a timer
					default:
						panic("should not be reached")
					}
					player.Status = status
				}
			}
		}
	}

	player.logPosition()
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

	return fmt.Sprintf("%d:%02d:%02d", hours, minutes, seconds)
}

func (player *Player) logPosition() {
	log.Printf("[DEBUG] current position: %s", FormatPosition(player.currentPosition()))
}

func (player *Player) logCurrentBookmark() {
	log.Printf("[DEBUG] bookmark: url=%s, position=%s", player.CurrentBookmark.Url, FormatPosition(player.CurrentBookmark.Position))
}

func (player *Player) setPosition(ms int64) {
	player.Position = ms
	player.PositionTime = time.Now()
}

func (player *Player) syncPosition(ms int64) error {
	if !player.HasTrackId {
		return errors.New("Player does not have a trackid")
	}

	if !player.TrackId.IsValid() {
		return errors.New(fmt.Sprintf("Player has an invalid trackid: '%s'", player.TrackId))
	}

	log.Printf("[DEBUG] Syncing player position to %s", FormatPosition(ms))
	err := player.MprisObj.Call("org.mpris.MediaPlayer2.Player.SetPosition", 0, player.TrackId, ms).Store()
	if err != nil {
		return err
	}
	player.setPosition(ms)
	return nil
}

func (player *Player) handleSeeked(message *dbus.Signal) {
	log.Printf("[DEBUG] handling seeked: %v", message)
	if seeked, ok := message.Body[0].(int64); ok {
		player.setPosition(seeked)
	} else {
		log.Printf("[DEBUG] got invalid seeked value")
	}

	player.logPosition()
}

func (player *Player) handlePropertiesChanged(message *dbus.Signal) {
	log.Printf("[DEBUG] handling properties changed: %v", message)
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

	log.Printf("[DEBUG] handling name owner changed: %v", message)

	if newOwner != player.NameOwner {
		// XXX: the name could actually be transferred, but I've never seen
		// this.
		log.Printf("[DEBUG] name owner changed from '%s' to '%s'", player.NameOwner, newOwner)
		return true
	}

	return false
}

func (player *Player) ensureCurrentBookmark() error {
	if !player.HasUrl {
		player.CurrentBookmark = nil
		return errors.New("player does not have url, no current bookmark")
	}

	if player.CurrentBookmark == nil || player.CurrentBookmark.Url != player.Url {
		bookmark, err := model.GetBookmark(player.DB, player.Url)
		if err != nil {
			return err
		}
		player.CurrentBookmark = bookmark
	}

	return nil
}

func (player *Player) restoreBookmark() error {
	log.Printf("[DEBUG] restoring bookmark")
	err := player.ensureCurrentBookmark()
	if err != nil {
		return err
	}
	player.logCurrentBookmark()

	if player.CurrentBookmark.Exists() {
		log.Printf("[DEBUG] bookmark exists, syncing to position %s", FormatPosition(player.CurrentBookmark.Position))
		err = player.syncPosition(player.CurrentBookmark.Position)
		if err != nil {
			return err
		}
	} else {
		log.Printf("[DEBUG] bookmark does not exist, not restoring")
	}

	return nil
}

func (player *Player) updateBookmark() error {
	log.Printf("[DEBUG] updating bookmark")
	err := player.ensureCurrentBookmark()
	if err != nil {
		return err
	}

	position := player.currentPosition()
	threshold := int64(1e+7)
	if position < threshold {
		log.Printf("[DEBUG] at the beginning threshold, deleting bookmark")
		return player.CurrentBookmark.Delete(player.DB)
	} else if player.Length > 0 && abs(player.Length-position) < threshold {
		log.Printf("[DEBUG] at the ending threshold, deleting bookmark")
		return player.CurrentBookmark.Delete(player.DB)
	}

	player.CurrentBookmark.Position = position
	player.logCurrentBookmark()
	log.Printf("[DEBUG] saving bookmark to position: %s", FormatPosition(position))
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
				log.Printf("[WARNING] could not send signal to player process: %v", err)
			}
		}
	}()
}

func (player *Player) Run() error {
	properties, err := player.getProperties()
	if err != nil {
		return err
	}

	player.setProperties(properties)
	player.installSignalHandlers()

	defer func() {
		err = player.updateBookmark()
		if err != nil {
			log.Printf("[DEBUG] could not update bookmark: %v", err)
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

	return player.Cmd.Wait()
}
