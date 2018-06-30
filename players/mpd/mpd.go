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

// encodeStatus gets the most relevant info from the passed Attrs struct
func encodeStatus(status m.Attrs, song m.Attrs) p.Status {
	return p.Status{
		Track: p.Track{
			Title:  song["Title"],
			Artist: song["Artist"],
			Album:  song["Album"],
		},
		CurrentStatus: p.CurrentStatus{
			Duration: status["duration"],
			Elapsed:  status["elapsed"],
			State:    status["state"],
		},
	}
}

// Watch monitors mpd for changes and posts the info to the channels.
func Watch(addr string) (chan p.Status, chan p.Status, chan error) {
	// create channels to keep track of the current statuses
	var automaticChan = make(chan p.Status)
	var explicitChan = make(chan p.Status)
	var errorChan = make(chan error)

	go func() {
		// Connect to mpd as a client.
		c, err := m.Dial("tcp", addr)
		if err != nil {
			errorChan <- err
		}

		// keep the connection alive
		keepAlive(c)

		// Create a watcher for its events
		w, err := m.NewWatcher("tcp", addr, "")
		if err != nil {
			errorChan <- err
		}

		// Watch for mpd's errors
		go func() {
			for err := range w.Error {
				errorChan <- err
			}
		}()

		go func() {
			for subsystem := range w.Event {
				// Watch for player changes
				if subsystem == "player" {
					status, err := c.Status()
					if err != nil {
						errorChan <- err
					}

					song, err := c.CurrentSong()
					if err != nil {
						errorChan <- err
					}

					s := encodeStatus(status, song)
					// only playing tracks matter
					if s.State == "play" {
						automaticChan <- s
					}
				} else {
					// other kinds of events aren't handled, so empty the channel
					<-w.Event
				}
			}
		}()

		go func() {
			for {
				status, err := c.Status()
				if err != nil {
					errorChan <- err
				}

				song, err := c.CurrentSong()
				if err != nil {
					errorChan <- err
				}

				s := encodeStatus(status, song)
				explicitChan <- s
			}
		}()

	}()

	return automaticChan, explicitChan, errorChan
}
