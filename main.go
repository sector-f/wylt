// Copyright (c) 2023 Luiz de Milon (kori)

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
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/google/go-cmp/cmp"
)

// struct for unmarshalling the config
type config struct {
	MPDAddress        string
	MPDPassword       string
	ListenbrainzToken string
}

// read configuration file and return a config struct
func newConfig(path string) (config, error) {
	// read config file
	configFile, err := os.ReadFile(path)
	if err != nil {
		return config{}, err
	}

	// parse config file and assign to a struct
	var c config
	_, err = toml.Decode(string(configFile), &c)
	if err != nil {
		return config{}, errors.New("config file not found")
	}
	return c, nil
}

// function to create a logger to both stdout and a *log.Logger
func newLogger(path string) *log.Logger {
	// open the logfile for appending or create it if it doesnt exist
	logfile, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatalln(err)
	}

	mw := io.MultiWriter(os.Stdout, logfile)
	return log.New(mw, "", log.LstdFlags)
}

// create a timer that will return whenever the submission time is elapsed
func newTimer(p Player, t Target, track Track) *time.Timer {
	// TODO: whenever I deal with concurrent logging, log these errors too
	st, _ := t.GetSubmissionTime(track.Duration)

	np, _ := p.NowPlaying()

	return time.AfterFunc(time.Duration(st)*time.Second, func() {
		if cmp.Equal(track, np) {
			t.SubmitListen(track)
		}
	})
}

func main() {
	// Set config root according to XDG standards.
	configRoot := filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "wylt")
	if configRoot == "" {
		configRoot = filepath.Join(os.Getenv("HOME"), ".config", "wylt")
	}

	configPath := filepath.Join(configRoot, "config.toml")
	config, err := newConfig(configPath)

	logPath := filepath.Join(configRoot, "logs", "wylt.log")
	logger := newLogger(logPath)
	if err != nil {
		logger.Fatalln(err)
	}

	mpdSession, err := NewMPD(config.MPDAddress, config.MPDPassword)
	if err != nil {
		logger.Fatalln(err)
	}

	ts := Targets{&listenbrainz{Token: config.ListenbrainzToken}}
	ps := Players{&mpdSession}

	var wg sync.WaitGroup

	// For each player...
	for _, p := range ps {
		// And for each target...
		for _, t := range ts {
			wg.Add(1)

			// ...create a function that will watch the player and submit it to the target
			go func(p Player, t Target) {
				defer wg.Done()

				playerLog, errs := p.Subscribe()
				for {
					select {
					// For each track change, create the timer that will handle submitting listens.
					case cur := <-playerLog:
						go func(p Player, t Target, track Track) {
							// TODO: at the time of this commit, this is printing twice. Check Subscribe()
							logger.Printf("[PLAYER]: now playing: %s by %s\n", track.Title, track.Artist)
							t.SubmitPlayingNow(track)
							newTimer(p, t, track)
						}(p, t, cur)
					case e := <-errs:
						// player errors are bound to fill up, should the connection be lost.
						// TODO: find a better way to deal with player error logs
						logger.Println(e)
					}
				}
			}(p, t)
		}
	}

	wg.Wait()
}
