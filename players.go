package main

import (
	"math"
	"strconv"
	"time"

	m "github.com/fhs/gompd/mpd"
)

// Track encodes information about a track.
type Track struct {
	Title    string
	Artist   string
	Album    string
	Duration time.Duration
}

// Player is an interface that encodes the methods that are used to get what Tracks we're supposed to submit.
type Player interface {
	// Subscribe will receive information about the player's current loaded track, and any errors.
	Subscribe() (chan Track, chan error)
	// NowPlaying checks what track is playing.
	NowPlaying() (Track, error)
}

type Players []Player

// mpd encodes information about what MPD instance Wylt will connect to.
type mpd struct {
	Client  *m.Client
	Watcher *m.Watcher
}

func NewMPD(address string, password string) (mpd, error) {
	c, err := m.DialAuthenticated("tcp", address, password)
	if err != nil {
		return mpd{}, err
	}

	// Create a watcher for its events
	w, err := m.NewWatcher("tcp", address, password)
	if err != nil {
		return mpd{}, err
	}

	return mpd{Client: c, Watcher: w}, nil
}

// TODO: bring client definition here and use it everywhere instead of dialing
// every time

// Subscribe will receive information about the player's Track, and any errors.
// Plus, there is an updater channel, so info can be requested manually.
func (m *mpd) Subscribe() (chan Track, chan error) {
	trackChannel := make(chan Track)
	errorChannel := make(chan error)

	// keep the connection alive
	go func() {
		for range time.Tick(30 * time.Second) {
			m.Client.Ping()
		}
	}()

	// Watch mpd's events
	go func() {
		for subsystem := range m.Watcher.Event {
			// Watch for player changes
			if subsystem == "player" {
				status, err := m.Client.Status()
				if err != nil {
					errorChannel <- err
				}

				// We need to call Status separately because the event watcher prints
				// out an event when the track starts, and when it starts playing and
				// also when it pauses, so that's too much noise for our purposes
				if status["state"] == "play" {
					t, err := m.NowPlaying()
					if err != nil {
						errorChannel <- err
					}

					trackChannel <- t
				}

			} else {
				// other kinds of events aren't handled, so empty the channel
				<-m.Watcher.Event
			}
		}
	}()

	return trackChannel, errorChannel
}

func (m *mpd) NowPlaying() (Track, error) {
	song, err := m.Client.CurrentSong()
	if err != nil {
		return Track{}, nil
	}

	status, err := m.Client.Status()
	if err != nil {
		return Track{}, nil
	}

	d, err := strconv.ParseFloat(status["duration"], 64)
	if err != nil {
		return Track{}, nil
	}

	return Track{
		Title:    song["Title"],
		Artist:   song["Artist"],
		Album:    song["Album"],
		Duration: time.Duration(int(math.Floor(d))) * time.Second,
	}, nil

}
