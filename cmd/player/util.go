package player

import (
	"fmt"
	"github.com/godbus/dbus/v5"
	"github.com/mitchellh/go-ps"
	"strings"
)

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

func ListPlayers(bus *dbus.Conn) ([]string, error) {
	prefix := "org.mpris.MediaPlayer2."
	players := []string{}
	names := []string{}
	err := bus.BusObject().Call("org.freedesktop.DBus.ListNames", 0).Store(&names)
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
