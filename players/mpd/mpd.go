package mpd

import (
	"time"

	m "github.com/fhs/gompd/mpd"
	p "github.com/kori/libra/players"
)

// keepAlive pings mpd every 30 seconds so the connection doesn't time out
func keepAlive(c *m.Client) {
	go func() {
		for range time.Tick(30 * time.Second) {
			c.Ping()
		}
	}()
}

// parseStatus gets the most relevant info from the passed Attrs struct
func parseStatus(status mpd.Attrs, song mpd.Attrs) p.Status {
	return p.Status{
		Title:    song["Title"],
		Artist:   song["Artist"],
		Album:    song["Album"],
		Duration: status["duration"],
		Elapsed:  status["elapsed"],
		State:    status["state"]}
}

// Watch monitors mpd for changes and posts the relevant info to the statusCh and errorCh channels
func Watch(addr string, statusCh chan<- p.Status, errorCh chan<- error) error {
	// Connect to mpd as a client.
	c, err := m.Dial("tcp", addr)
	if err != nil {
		return err
	}
	// keep the connection alive
	keepAlive(c)

	// Create a watcher for its events
	w, err := m.NewWatcher("tcp", addr, "")
	defer w.Close()
	if err != nil {
		return err
	}

	for subsystem := range w.Event {
		// Watch for player changes
		if subsystem == "player" {
			status, err := c.Status()
			if err != nil {
				return err
			}
			song, err := c.CurrentSong()
			if err != nil {
				return err
			}

			s := parseStatus(status, song)
			// only playing tracks matter
			if s.State == "play" {
				statusCh <- s
			}
		} else {
			// other kinds of events aren't handled, so empty the event channel
			<-w.Event
		}
	}

	// Watch for mpd's errors
	go func() {
		for err := range w.Error {
			go func() {
				errorCh <- err
			}()
		}
	}()

	return nil
}
