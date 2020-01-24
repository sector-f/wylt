package main

import (
	"math"
	"strconv"
	"time"

	m "github.com/fhs/gompd/mpd"
)

// Track encodes information about a track.
type Track struct {
	Title  string
	Artist string
	Album  string
}

// playerStatus encodes information about the player.
type playerStatus struct {
	Track           // Current loaded track
	Duration int    // Total duration of the current Track
	State    string // Whether the player is playing or paused
}

// mpd encodes information about what MPD instance Wylt will connect to.
type mpd struct {
	Address  string
	Password string
}

// Subscribe will receive information about the player's status, and any errors.
// Plus, there is an updater channel, so info can be requested manually.
func (session *mpd) Subscribe() (chan playerStatus, chan error) {
	statusChannel := make(chan playerStatus)
	errorChannel := make(chan error)

	// Connect to mpd as a client.
	// If the password is empty, a regular mpd.Dial command will be issued.
	c, err := m.DialAuthenticated("tcp", session.Address, session.Password)
	if err != nil {
		errorChannel <- err
	}

	// keep the connection alive
	go func() {
		for range time.Tick(30 * time.Second) {
			c.Ping()
		}
	}()

	// Create a watcher for its events
	w, err := m.NewWatcher("tcp", session.Address, "")
	if err != nil {
		errorChannel <- err
	}

	// Watch mpd's events
	go func() {
		for subsystem := range w.Event {
			// Watch for player changes
			if subsystem == "player" {
				status, err := c.Status()
				if err != nil {
					errorChannel <- err
				}

				// only playing tracks matter
				if status["state"] == "play" {
					song, err := c.CurrentSong()
					if err != nil {
						errorChannel <- err
					}

					de, err := strconv.ParseFloat(status["duration"], 64)
					if err != nil {
						errorChannel <- err
					}
					duration := int(math.Floor(de))

					statusChannel <- playerStatus{
						Track: Track{
							Title:  song["Title"],
							Artist: song["Artist"],
							Album:  song["Album"],
						},
						Duration: duration,
						State:    status["state"],
					}
				}
			} else {
				// other kinds of events aren't handled, so empty the channel
				<-w.Event
			}
		}
	}()

	return statusChannel, errorChannel
}

func (session *mpd) NowPlaying() (Track, error) {
	// Connect to mpd as a client.
	c, err := m.Dial("tcp", session.Address)
	if err != nil {
		return Track{}, err
	}

	song, err := c.CurrentSong()
	if err != nil {
		return Track{}, err
	}

	return Track{
		Title:  song["Title"],
		Artist: song["Artist"],
		Album:  song["Album"],
	}, nil
}
