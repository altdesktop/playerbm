package player

import (
	"database/sql"
	"github.com/altdesktop/playerbm/cmd/cli"
	"github.com/altdesktop/playerbm/cmd/model"
	"github.com/godbus/dbus/v5"
	"os/exec"
	"time"
)

const (
	Playing = "Playing"
	Paused  = "Paused"
	Stopped = "Stopped"
)

type Player struct {
	DB            *sql.DB
	Bus           *dbus.Conn
	Cli           *cli.PbmCli
	Cmd           *exec.Cmd
	Bookmark      *model.Bookmark
	BusName       string
	NameOwner     string
	MprisObj      dbus.BusObject
	Position      int64
	PositionTime  time.Time
	TrackId       dbus.ObjectPath
	Status        string
	Length        int64
	ProcessFinish chan error
	Signals       chan *dbus.Signal
	ExitCode      int
}

func New(cli *cli.PbmCli, db *sql.DB, bus *dbus.Conn) *Player {
	return &Player{
		Cli:           cli,
		DB:            db,
		Bus:           bus,
		Status:        Stopped,
		ProcessFinish: make(chan error),
		Signals:       make(chan *dbus.Signal, 10),
		PositionTime:  time.Now(),
	}
}

type PlayerCmdError struct {
	err      string
	ExitCode int
}

func (e *PlayerCmdError) Error() string {
	return e.err
}

type Properties struct {
	Position    int64
	HasPosition bool
	Length      int64
	HasLength   bool
	Url         *model.XesamUrl
	Status      string
	TrackId     dbus.ObjectPath
}
