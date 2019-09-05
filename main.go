// Copyright (c) 2018 Luiz de Milon (kori)

// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.

// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.

// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package main

import (
	"errors"
	"io"
	"io/ioutil"
	"log"
	"math"
	"os"
	"strconv"
	"time"

	"github.com/BurntSushi/toml"
	lb "github.com/kori/go-listenbrainz"
)

// struct for unmarshalling the config
type config struct {
	MPDAddress        string
	ListenbrainzToken string
}

type Track struct {
	Title  string
	Artist string
	Album  string
}

type CurrentStatus struct {
	Duration int
	Elapsed  int
	State    string
}

// Status is a struct for encoding the current state of the player
type Status struct {
	Track
	CurrentStatus
}

// encodeStatus gets the most relevant info from the passed Attrs struct
func encodeStatus(status m.Attrs, song m.Attrs) (Status, error) {
	fe, err := strconv.ParseFloat(status["elapsed"], 64)
	if err != nil {
		return Status{}, err
	}
	elapsed := int(math.Floor(fe))

	de, err := strconv.ParseFloat(status["duration"], 64)
	if err != nil {
		return Status{}, err
	}
	duration := int(math.Floor(de))

	return Status{
		Track: Track{
			Title:  song["Title"],
			Artist: song["Artist"],
			Album:  song["Album"],
		},
		CurrentStatus: CurrentStatus{
			Duration: duration,
			Elapsed:  elapsed,
			State:    status["state"],
		},
	}, nil
}

// read configuration file and return a config struct
func getConfig(path string) (config, error) {
	// read config file
	configFile, err := ioutil.ReadFile(path)
	if err != nil {
		return config{}, err
	}

	// parse config file and assign to a struct
	var c config
	_, err = toml.Decode(string(configFile), &c)
	if err != nil {
		return config{}, errors.New("Config file not found.")
	}
	return c, nil
}

func main() {
	logfile, err := os.Create(logroot + "/" + "libra-" + strconv.FormatInt(time.Now().Unix(), 10))
	if err != nil {
		fmt.Fatalln(err)
	}

	mw := io.MultiWriter(os.Stdout, logfile)
	// Create a logger for libra.
	l, err := log.New(mw, "", log.LstdFlags)
	if err != nil {
		fmt.Fatalln(err)
	}

	// Set config home according to XDG standards.
	configHome := os.Getenv("XDG_CONFIG_HOME")
	if configHome == "" {
		configHome = os.Getenv("HOME") + "/.config"
	}

	// Create subdirectories.
	configroot := configHome + "/libra"
	logroot := configroot + "/log"

	// get config info from the path
	conf, err := getConfig(configroot + "/config.toml")
	if err != nil {
		l.Fatalln(err)
	}

	// watch the errors channel
	go func() {
		for err := range w.Event {
			l.Println(err)
		}
	}()

	// get initial status.
	initial := <-playingNow
	track := current.Title + " by " + current.Artist + " on " + current.Album
	l.Println("mpd: Playing now:", track)

	// post current track to listenbrainz
	r, err := lb.SubmitPlayingNow(lb.Track(current.Track), conf.ListenbrainzToken)
	if err != nil {
		l.Fatalln(err)
	}
	l.Println("listenbrainz:", r.Status+":", "Playing now:", track)

	st, err := lb.GetSubmissionTime(current.Duration)
	if err != nil {
		l.Fatalln(err)
	}

	// submit the track if the submission time has elapsed and if it's still the same track
	time.AfterFunc(time.Duration(st)*time.Second, func() {
		new := <-playingNow
		if current.Track == new.Track {
			r, err := lb.SubmitSingle(lb.Track(current.Track), conf.ListenbrainzToken, time.Now().Unix())
			if err != nil {
				l.Fatalln(err)
			}
			l.Println("listenbrainz:", r.Status+":", "Single submission:", track)
		}
	})

	// Connect to mpd as a client.
	c, err := m.Dial("tcp", addr)
	if err != nil {
		l.Fatalln("libra: Couldn't connect to mpd.")
		l.Fatalln("libra:", err)
	}

	// keep the connection alive
	go func() {
		for range time.Tick(30 * time.Second) {
			c.Ping()
		}
	}()

	// Create a watcher for its events
	w, err := m.NewWatcher("tcp", addr, "")
	if err != nil {
		l.Fatalln("mpd: watcher:", err)
	}

	// Watch for mpd's errors
	go func() {
		for err := range w.Error {
			l.Println("libra:", err)
		}
	}()

	// Watch mpd's events
	go func() {
		for subsystem := range w.Event {
			// Watch for player changes
			if subsystem == "player" {
				status, err := c.Status()
				if err != nil {
					l.Println("libra:", err)
				}

				// only playing tracks matter
				if status["state"] == "play" {
					song, err := c.CurrentSong()
					if err != nil {
						l.Println("libra:", err)
					}
					s, err := encodeStatus(status, song)
					if err != nil {
						l.Println("libra:", err)
					}
				}
			} else {
				// other kinds of events aren't handled, so empty the channel
				<-w.Event
			}
		}
	}()

	// Watch for manual requests
	status, err := c.Status()
	if status["state"] == "play" {
		if err != nil {
			l.Println("libra:", err)
		}

		song, err := c.CurrentSong()
		if err != nil {
			l.Println("libra:", err)
		}

		s, err := encodeStatus(status, song)
		if err != nil {
			l.Println("libra:", err)
		}
	}

}
