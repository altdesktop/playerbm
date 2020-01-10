# 📚 🎧 playerbm

Bookmark CLI for media players to resume where you left off in audiobooks and podcasts.

## About

playerbm is a command-line utility that saves your place when you exit the player or change the track and automatically resumes from where you left off when you open it again. This is useful if you listen to long audiobooks or lectures over many sessions.

Pass the command to open your media player to playerbm and it will connect to the player begin managing bookmarks.

```
# Open your audiobook with mpv
playerbm mpv ~/audiobooks/war-and-peace.mp3
```

If you've opened the file with playerbm before, it should seek to the last known position. When you exit the player, it will save a bookmark and open the file to that location next time.

To list all the bookmarks that playerbm is managing, use the `--list-bookmarks` flag.

```
# Print a readable list of all your bookmarks
playerbm --list-bookmarks
```

To resume playback from the last bookmark that was saved, use the `--resume` flag. This will open the last saved url in a player that is playing the file or open a new player with the default media player using `xdg-open` (usually provided by the package `xdg-utils`). You can pass a `FILE` to the `--resume` flag to resume playing from the last bookmark for a particular file. For some help on setting a default media player, see [this Gist](https://gist.github.com/acrisci/b264c4b8e7f93a21c13065d9282dfa4a).

```
# Resume playing the last opened bookmark
playerbm --resume

# Resume playing the last opened bookmark for your podcast
playerbm --resume ~/podcasts/true-crime.mp3
```

To save a bookmark when playerbm is not managing the player, you can use the `--save` flag to save bookmarks for all running media players. If `PLAYER` is passed as a comma separated list, it will only save bookmarks for those players. You can see what players can be connected to with the `--list-players` flag.

```
# Save bookmarks for your running media players
playerbm --save

# Print all the players running on your system
playerbm --list-players

# Only save a bookmark for mpv
playerbm --save mpv
```

## Installing

```
go get -u github.com/altdesktop/playerbm
```

## Player Support

playerbm should support any media player that implements the [MPRIS D-Bus Interface Specification](https://specifications.freedesktop.org/mpris-spec/latest/). If your player does not work well with playerbm, open an issue on Github and I'll look into supporting it. Contributions are welcome.

Known working players:

* [mpv](https://github.com/mpv-player/mpv) with [mpv-mpris](https://github.com/hoyon/mpv-mpris) plugin
* [smplayer](https://www.smplayer.info/)

## License

You can use this code under an MIT license (see LICENSE).

© 2019, Tony Crisci
