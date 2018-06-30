package mpd

import (
	"math"
	"strconv"
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
func encodeStatus(status m.Attrs, song m.Attrs) (p.Status, error) {
	fe, err := strconv.ParseFloat(status["elapsed"], 64)
	if err != nil {
		return p.Status{}, err
	}
	elapsed := int(math.Floor(fe))

	de, err := strconv.ParseFloat(status["duration"], 64)
	if err != nil {
		return p.Status{}, err
	}
	duration := int(math.Floor(de))

	return p.Status{
		Track: p.Track{
			Title:  song["Title"],
			Artist: song["Artist"],
			Album:  song["Album"],
		},
		CurrentStatus: p.CurrentStatus{
			Duration: duration,
			Elapsed:  elapsed,
			State:    status["state"],
		},
	}, nil
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
				// empty the explicit request channel
				<-explicitChan
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

					s, err := encodeStatus(status, song)
					if err != nil {
						errorChan <- err
					}

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

				s, err := encodeStatus(status, song)
				if err != nil {
					errorChan <- err
				}
				explicitChan <- s
			}
		}()

	}()

	return automaticChan, explicitChan, errorChan
}
