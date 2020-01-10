package player

import (
	"errors"
	"fmt"
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

const mprisPrefix = "org.mpris.MediaPlayer2."
const mprisPath = "/org/mpris/MediaPlayer2"

func parseProperties(propertiesVariant map[string]dbus.Variant) *Properties {
	properties := Properties{}

	// get the position
	if variant, found := propertiesVariant["Position"]; found {
		if val, ok := variant.Value().(int64); ok {
			properties.Position = val
			properties.HasPosition = true
		}
	}

	if metadataVariant, found := propertiesVariant["Metadata"]; found {
		if metadata, ok := metadataVariant.Value().(map[string]dbus.Variant); ok {
			// get the length
			if variant, ok := metadata["mpris:length"]; ok {
				if val, ok := variant.Value().(int64); ok {
					properties.HasLength = true
					properties.Length = val
				}
			}

			// get the trackid
			if variant, found := metadata["mpris:trackid"]; found {
				if val, ok := variant.Value().(dbus.ObjectPath); ok {
					properties.TrackId = val
				}
			}

			// get the url
			if variant, found := metadata["xesam:url"]; found {
				if val, ok := variant.Value().(string); ok {
					normalizedUrl, err := model.ParseXesamUrl(val)
					if err != nil {
						log.Printf("[DEBUG] player gave invalid url: %s", val)
					} else {
						properties.Url = normalizedUrl
					}
				}
			}
		}
	}

	// get the playback status
	if variant, found := propertiesVariant["PlaybackStatus"]; found {
		if val, ok := variant.Value().(string); ok {
			properties.Status = val
		}
	}

	return &properties
}

func (player *Player) GetPropertiesRemote() (*Properties, error) {
	var propertiesVariant map[string]dbus.Variant
	err := player.MprisObj.Call("org.freedesktop.DBus.Properties.GetAll", dbus.FlagNoAutoStart, "org.mpris.MediaPlayer2.Player").Store(&propertiesVariant)
	if err != nil {
		return nil, err
	}

	return parseProperties(propertiesVariant), nil
}

func (player *Player) SetPlayerProperties(properties *Properties) {
	player.Status = properties.Status
	player.TrackId = properties.TrackId
	if properties.HasLength {
		player.Length = properties.Length
	}
	if properties.HasPosition {
		player.setPosition(properties.Position)
	}
}

func (player *Player) syncBookmark(properties *Properties) {
	var queueUpdate bool

	if len(properties.TrackId) > 0 {
		player.TrackId = properties.TrackId
	}

	currentUrl := player.currentUrl()
	if properties.Url != nil && (currentUrl == nil || properties.Url.String() != currentUrl.String()) {
		log.Printf("[DEBUG] url has changed from '%s' to '%s'", currentUrl, properties.Url)
		err := player.updateBookmark()
		if err != nil {
			log.Printf("[DEBUG] could not update current bookmark: %+v", err)
		}
		err = player.LoadBookmark(properties.Url)
		if err != nil {
			log.Printf("[DEBUG] could not load bookmark: %+v", err)
		}
		queueUpdate = true
	}

	if properties.HasLength && player.Bookmark != nil && player.Bookmark.Length != properties.Length {
		log.Printf("[DEBUG] setting player length to '%s'", FormatPosition(properties.Length))
		player.Bookmark.Length = properties.Length
	}

	if len(properties.Status) > 0 && properties.Status != player.Status {
		log.Printf("[DEBUG] playback status has changed from '%s' to '%s'", player.Status, properties.Status)
		switch properties.Status {
		case Playing:
			player.PositionTime = time.Now()
		case Paused:
		case Stopped:
			// TODO: no track currently playing if stopped
			player.Position = player.currentPosition()
		default:
			log.Printf("[DEBUG] player gave invalid status: %s", properties.Status)
		}
		queueUpdate = true
		player.Status = properties.Status
	}

	if properties.HasPosition {
		log.Printf("[DEBUG] position has changed from '%s' to '%s'", FormatPosition(player.currentPosition()), FormatPosition(properties.Position))
		player.setPosition(properties.Position)
	}

	player.logPosition()
	player.logCurrentBookmark()

	if queueUpdate {
		// Run this if anything important has changed. This works around spec
		// weirdness regarding position.
		go func() {
			properties, err := player.GetPropertiesRemote()
			if err != nil {
				log.Printf("[DEBUG] could not get properties: %+v", err)
				return
			}

			player.syncBookmark(properties)
		}()
	}
}

func (player *Player) currentUrl() *model.XesamUrl {
	if player.Bookmark == nil {
		return nil
	}

	return player.Bookmark.Url
}

func (player *Player) currentPosition() int64 {
	if player.Status == Playing {
		return player.Position + time.Since(player.PositionTime).Microseconds()
	} else {
		return player.Position
	}
}

func (player *Player) logPosition() {
	log.Printf("[DEBUG] current position: %s", FormatPosition(player.currentPosition()))
}

func (player *Player) logCurrentBookmark() {
	if player.Bookmark == nil {
		log.Printf("[DEBUG] no current bookmark")
	} else {
		log.Printf("[DEBUG] bookmark: url=%s, position=%s", player.Bookmark.Url, FormatPosition(player.Bookmark.Position))
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
	err := player.MprisObj.Call("org.mpris.MediaPlayer2.Player.Play", dbus.FlagNoAutoStart).Store()
	if err != nil {
		return err
	}
	err = player.MprisObj.Call("org.mpris.MediaPlayer2.Player.SetPosition", dbus.FlagNoAutoStart, player.TrackId, ms).Store()
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
		player.syncBookmark(parseProperties(properties))
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

func (player *Player) LoadBookmark(url *model.XesamUrl) error {
	player.logCurrentBookmark()

	if player.Bookmark != nil && player.Bookmark.Url == url {
		log.Printf("[DEBUG] url unchanged, not loading bookmark")
		return nil
	}

	bookmark, err := model.GetBookmark(player.DB, url)
	if err != nil {
		return err
	}

	if bookmark.Exists() {
		log.Printf("[DEBUG] bookmark exists, syncing to position %s", FormatPosition(bookmark.Position))
		//time.Sleep(100 * time.Millisecond)
		err = player.syncPosition(bookmark.Position)
		if err != nil {
			log.Printf("[DEBUG] could not sync position: %+v", err)
		}
	} else {
		log.Printf("[DEBUG] bookmark does not exist, not restoring")
	}

	player.Bookmark = bookmark
	player.logCurrentBookmark()

	return nil
}

func (player *Player) updateBookmark() error {
	if player.Bookmark == nil {
		log.Printf("[DEBUG] no current bookmark to update")
		return nil
	}

	position := player.currentPosition()
	log.Printf("[DEBUG] saving bookmark to position: %s", FormatPosition(position))
	player.Bookmark.Position = position
	player.logCurrentBookmark()
	return player.Bookmark.Save(player.DB)
}

var signalHandlersInstalled bool

func (player *Player) installSignalHandlers() {
	if signalHandlersInstalled {
		return
	}

	go func() {
		signals := make(chan os.Signal, 10)
		signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
		for {
			s := <-signals
			if player.Cmd != nil {
				err := player.Cmd.Process.Signal(s)
				if err != nil {
					log.Printf("[WARNING] could not send signal to player process: %+v", err)
					player.ExitCode = 130
					player.Signals <- nil
				}
			} else {
				player.ExitCode = 130
				player.Signals <- nil
			}
		}
	}()

	signalHandlersInstalled = true
}

var matchSignalAdded bool

func (player *Player) addNameOwnerChangedMatchSignal() error {
	if matchSignalAdded {
		return nil
	}

	err := player.Bus.AddMatchSignal(
		dbus.WithMatchSender("org.freedesktop.DBus"),
		dbus.WithMatchInterface("org.freedesktop.DBus"),
		dbus.WithMatchObjectPath("/org/freedesktop/DBus"),
		dbus.WithMatchMember("NameOwnerChanged"),
	)
	if err != nil {
		return err
	}

	matchSignalAdded = true
	return nil
}

func (player *Player) initProcess() error {
	busObj := player.Bus.BusObject()

	err := player.addNameOwnerChangedMatchSignal()
	if err != nil {
		return err
	}

	// listen to any property changed signals of media players
	playerPropertiesMatch := []dbus.MatchOption{
		dbus.WithMatchInterface("org.freedesktop.DBus.Properties"),
		dbus.WithMatchObjectPath(mprisPath),
		dbus.WithMatchMember("PropertiesChanged"),
	}
	err = player.Bus.AddMatchSignal(playerPropertiesMatch...)
	if err != nil {
		return err
	}
	defer player.Bus.RemoveMatchSignal(playerPropertiesMatch...)

	signals := make(chan *dbus.Signal, 10)
	player.Bus.Signal(signals)
	defer player.Bus.RemoveSignal(signals)

	log.Printf("[DEBUG] %s", player.Cli.PlayerCmd)
	player.Cmd = exec.Command("/bin/bash", "-c", player.Cli.PlayerCmd)
	player.Cmd.Stdout = os.Stdout
	player.Cmd.Stderr = os.Stderr

	go func() {
		err = player.Cmd.Run()
		if player.Cmd != nil {
			player.ExitCode = player.Cmd.ProcessState.ExitCode()
		}

		player.ProcessFinish <- err
	}()

	timeout := make(chan error, 10)
	timeoutSeconds := time.Duration(20)
	go func() {
		time.Sleep(timeoutSeconds * time.Second)
		if timeout != nil {
			timeout <- errors.New("timeout")
		}
	}()
	defer func() {
		close(timeout)
		timeout = nil
	}()

	signalHandler := func(message *dbus.Signal) bool {
		log.Printf("[DEBUG] got signal: %+v", message)
		if message.Name == "org.freedesktop.DBus.NameOwnerChanged" {
			name := fmt.Sprintf("%s", message.Body[0])
			// oldOwner := fmt.Sprintf("%s", message.Body[1])
			newOwner := fmt.Sprintf("%s", message.Body[2])

			if !strings.HasPrefix(name, mprisPrefix) {
				return false
			}

			if len(newOwner) > 0 {
				log.Printf("[DEBUG] a player appeared: name: %s, owner: %s", name, newOwner)
				var pid int
				err := busObj.Call("org.freedesktop.DBus.GetConnectionUnixProcessID", dbus.FlagNoAutoStart, name).Store(&pid)
				if err != nil {
					log.Printf("[DEBUG] could not get process id: %+v", err)
					return false
				}
				log.Printf("[DEBUG] pid: %d", pid)
				processMatch, err := isChildProcess(player.Cmd.Process.Pid, pid)
				if err != nil {
					log.Printf("[DEBUG] could not get process info: %+v", err)
					return false
				}
				if processMatch {
					log.Printf("[DEBUG] managing player by process id detection")
					player.BusName = name
					player.NameOwner = newOwner
					player.MprisObj = player.Bus.Object(name, mprisPath)
					return true
				}
			}
		} else if message.Name == "org.freedesktop.DBus.Properties.PropertiesChanged" && message.Path == mprisPath {
			if player.Cli.ResumeFile == nil {
				// we can't handle this signal without a file to resume
				return false
			}

			iface := fmt.Sprintf("%s", message.Body[0])
			if iface != "org.mpris.MediaPlayer2.Player" {
				return false
			}
			if propertiesVariant, ok := message.Body[1].(map[string]dbus.Variant); ok {
				properties := parseProperties(propertiesVariant)
				if properties.Url != nil && properties.Url.String() == player.Cli.ResumeFile.String() {
					log.Printf("[DEBUG] matched a player by url detection")
					// We have to find the name for this sender. This the best
					// way I can think of doing it right now, but it's a lot of
					// dbus calls.
					players, err := ListPlayers(player.Bus)
					if err != nil {
						log.Printf("[DEBUG] could not list names: %+v", err)
						return false
					}

					for _, name := range players {
						busName := fmt.Sprintf("%s%s", mprisPrefix, name)
						var nameOwner string
						err = busObj.Call("org.freedesktop.DBus.GetNameOwner", 0, busName).Store(&nameOwner)
						if err != nil {
							log.Printf("[DEBUG] could not get name owner for: %s", busName)
							continue
						}
						if nameOwner == message.Sender {
							log.Printf("[DEBUG] managing player with bus name: %s", busName)
							player.BusName = busName
							player.NameOwner = nameOwner
							player.MprisObj = player.Bus.Object(busName, mprisPath)
							return true
						}
					}

					return false
				}
			}
		}

		return false
	}

loop:
	for {
		select {
		case message := <-signals:
			if signalHandler(message) {
				break loop
			}
		case err = <-player.ProcessFinish:
			close(player.ProcessFinish)
			player.ProcessFinish = nil
			break loop
		case err = <-timeout:
			break loop
		}
	}

	if player.MprisObj != nil {
		return nil
	}

	if player.Cmd.ProcessState != nil && player.Cmd.ProcessState.Exited() {
		exitCode := player.Cmd.ProcessState.ExitCode()

		if exitCode != 0 || player.Cli.ResumeFile == nil {
			return &PlayerCmdError{
				err:      fmt.Sprintf("player process exited unexpectedly (exit %d)", exitCode),
				ExitCode: exitCode,
			}
		}

	loop2:
		for {
			select {
			case message := <-signals:
				if signalHandler(message) {
					break loop2
				}
			case err = <-timeout:
				break loop2
			}
		}

		if player.MprisObj == nil {
			return &PlayerCmdError{
				err:      fmt.Sprintf("player did not start within %d seconds", timeoutSeconds),
				ExitCode: 1,
			}
		} else {
			return nil
		}
	} else {
		return &PlayerCmdError{
			err:      fmt.Sprintf("player did not start within %d seconds", timeoutSeconds),
			ExitCode: 1,
		}
	}
}

func (player *Player) RunCmd() error {
	err := player.initProcess()
	if err != nil {
		return err
	}
	properties, err := player.GetPropertiesRemote()
	if err != nil {
		return err
	}

	player.syncBookmark(properties)

	err = player.Manage()
	if err != nil {
		return err
	}

	if player.ProcessFinish != nil {
		return <-player.ProcessFinish
	} else {
		return nil
	}
}

func (player *Player) HasBookmark() bool {
	return player.Bookmark != nil
}

func (player *Player) SaveBookmark() error {
	if player.Bookmark != nil {
		return player.Bookmark.Save(player.DB)
	}

	return errors.New("player does not have a bookmark to save")
}

func (player *Player) SetName(name string) {
	if !strings.HasPrefix(name, mprisPrefix) {
		name = fmt.Sprintf("%s%s", mprisPrefix, name)
	}

	log.Printf("[DEBUG] loading player named: %s", name)

	player.BusName = name
	player.MprisObj = player.Bus.Object(name, mprisPath)
}

func (player *Player) EnsureBookmark() error {
	var err error

	properties, err := player.GetPropertiesRemote()
	if err != nil {
		return err
	}

	player.setPosition(properties.Position)
	if properties.HasLength {
		player.Length = properties.Length
	}
	player.Status = properties.Status
	player.TrackId = properties.TrackId

	if properties.Url != nil {
		bookmark, err := model.GetBookmark(player.DB, properties.Url)
		if err != nil {
			return err
		}
		if properties.HasLength {
			bookmark.Length = properties.Length
		}
		player.Bookmark = bookmark
	} else {
		return errors.New("player does not have a valid url")
	}

	return nil
}

func (player *Player) Manage() error {
	player.installSignalHandlers()
	err := player.addNameOwnerChangedMatchSignal()
	if err != nil {
		return err
	}

	if len(player.NameOwner) == 0 {
		err = player.Bus.BusObject().Call(
			"org.freedesktop.DBus.GetNameOwner", 0, player.BusName,
		).Store(&player.NameOwner)
		if err != nil {
			return err
		}
	}

	// start the main loop
	player.Bus.Signal(player.Signals)
	defer player.Bus.RemoveSignal(player.Signals)

	err = player.Bus.AddMatchSignal(
		dbus.WithMatchSender(player.NameOwner),
		dbus.WithMatchObjectPath(mprisPath),
	)
	if err != nil {
		return err
	}

	for message := range player.Signals {
		if message == nil {
			break
		}

		if message.Sender == player.NameOwner && message.Path == mprisPath {
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

	err = player.updateBookmark()
	if err != nil {
		return err
	}

	return nil
}
