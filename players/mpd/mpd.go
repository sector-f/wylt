package mpd

import (
	"errors"
	"log"
	"time"

	m "github.com/fhs/gompd/mpd"
	p "github.com/kori/libra/players"
)

// a function to connect to mpd and return the client
func connect(addr string) (*m.Client, error) {
	// Connect to mpd as a client.
	c, err := m.Dial("tcp", addr)
	if err != nil {
		return &m.Client{}, err
	}

	// keep the connection alive
	keepAlive(c)

	return c, nil
}

// a function to keep the mpd connection alive
func keepAlive(c *m.Client) {
	go func() {
		for range time.Tick(30 * time.Second) {
			c.Ping()
		}
	}()
}

// GetStatus queries mpd and returns a Status struct with the current state
func GetStatus(c *m.Client) (p.Status, error) {
	status, err := c.Status()
	if err != nil {
		return p.Status{}, err
	}

	if status["state"] == "play" || status["state"] == "pause" {
		// query mpd for the info about the current song
		song, err := c.CurrentSong()
		if err != nil {
			return p.Status{}, err
		}

		return p.Status{
			Title:    song["Title"],
			Artist:   song["Artist"],
			Album:    song["Album"],
			Duration: status["duration"],
			Elapsed:  status["elapsed"],
			State:    status["state"]}, nil
	}

	return p.Status{}, errors.New("MPD is not playing anything")
}

// Watch monitors mpd for changes and posts the relevant info to the statusCh and errorCh channels
func Watch(addr string, statusCh chan<- p.Status, errorCh chan<- error) {
	// Connect to mpd
	c, err := connect(addr)
	defer c.Close()
	if err != nil {
		log.Fatalln(err)
	}
	// Create a watcher for its events
	w, err := m.NewWatcher("tcp", addr, "")
	defer w.Close()
	if err != nil {
		log.Fatalln("Failed to create a watcher: ", err)
	}

	// Watch for player changes
	for subsystem := range w.Event {
		if subsystem == "player" {
			// Connect to mpd to get the current track
			s, err := GetStatus(c)
			// if there's anything playing, log it
			if err == nil && s.State == "play" {
				statusCh <- s
			}
		} else {
			// empty the event channel so the program doesn't lock up
			<-w.Event
		}
	}

	// Watch for errors
	go func() {
		for err := range w.Error {
			errorCh <- err
		}
	}()
}
