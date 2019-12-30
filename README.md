# playerbm

A bookmark utility for media players.

## About

playerbm is a utility that saves your place when you exit the player or change the track and automatically resumes from where you left off when you open it again. This is useful if you listen to long audiobooks or lectures over many sessions.

Pass the command to open your media player to playerbm and it will connect to the player begin managing bookmarks.

```
playerbm mpv ~/audiobooks/war-and-peace.mp3
```

If you've opened the file with playerbm before, it should seek to the last known position. When you exit the player, it will save a bookmark and open the file to that location next time.

## Installing

```
go get -u github.com/altdesktop/playerbm
```

## Player Support

playerbm should support any media player that implements the [MPRIS D-Bus Interface Specification](https://specifications.freedesktop.org/mpris-spec/latest/). If your player does not work well with playerbm, open an issue on Github and I'll look into supporting it. Contributions are welcome.

Known working players:

* [mpv](https://github.com/mpv-player/mpv) with [mpv-mpris](https://github.com/hoyon/mpv-mpris) plugin.

## License

You can use this code under an MIT license (see LICENSE).

© 2019, Tony Crisci
