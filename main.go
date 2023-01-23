// Copyright (c) 2020 Luiz de Milon (kori)

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
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/google/go-cmp/cmp"
)

// struct for unmarshalling the config
type config struct {
	MPDAddress        string
	ListenbrainzToken string
}

// Player is an interface that encodes what works as a source of tracks.
// In this case, it could be any Subscribe() function that returns a Status
type Player interface {
	// Subscribe will receive information about the player's status, and any errors.
	Subscribe() (chan playerStatus, chan error)
	// NowPlaying checks what track is playing.
	NowPlaying() (Track, error)
}

// Players is the array where information is going to be collected from.
type Players []Player

// Target is an interface that encodes what works as a target.
// In this case, it can be anything that has a Publish() function that returns a http response.
type Target interface {
	// SubmitPlayingNow wraps a target's "playing now" function. (It's used in last.fm, libre.fm, and listenbrainz.)
	SubmitPlayingNow(Track) (*http.Response, error)
	// SubmitListen says you've listened to a track, according to a Target's parameters on what counts as a listen.
	SubmitListen(Track) (*http.Response, error)
	// GetSubmissionTime says when a listen should be submitted.
	GetSubmissionTime(int) (int, error)
}

// Targets is an array of Target, which means the Router can send information to multiple Targets.
type Targets []Target

func main() {
	// Set config home according to XDG standards.
	configHome := os.Getenv("XDG_CONFIG_HOME")
	if configHome == "" {
		configHome = os.Getenv("HOME") + "/.config"
	}
	// Create subdirectories.
	configroot := configHome + "/wylt"
	logroot := configroot + "/logs"

	logger := createLogger(logroot + "/" + "wylt-" + strconv.FormatInt(time.Now().Unix(), 10))

	// get config info from the path
	config, err := getConfig(configroot + "/config.toml")
	if err != nil {
		logger.Fatalln(err)
	}

	ts := Targets{&listenbrainz{Token: config.ListenbrainzToken}}
	ps := Players{&mpd{Address: config.MPDAddress}}

	var wg sync.WaitGroup
	for _, p := range ps {
		for _, t := range ts {
			wg.Add(1)

			go func(p Player, t Target) {
				defer wg.Done()

				playerLog, errs := p.Subscribe()
				for {
					select {
					case cur := <-playerLog:
						go func(p Player, t Target, status playerStatus) {
							logger.Printf("now playing: %s by %s\n", status.Track.Title, status.Track.Artist)
							t.SubmitPlayingNow(status.Track)
							createTimer(p, t, status)
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

// create a timer that will return whenever the submission time is elapsed
func createTimer(p Player, t Target, ps playerStatus) *time.Timer {
	// get submission time for the given track
	// TODO: whenever I deal with concurrent logging, log this error too
	st, _ := t.GetSubmissionTime(ps.Duration)

	return time.AfterFunc(time.Duration(st)*time.Second, func() {
		cur, _ := p.NowPlaying()
		// Is the same track still playing?
		if cmp.Equal(ps.Track, cur) {
			t.SubmitListen(ps.Track)
		}
	})
}

// function to create a logger to both stdout and a *log.Logger
func createLogger(path string) *log.Logger {
	logfile, err := os.Create(path)
	if err != nil {
		log.Fatalln(err)
	}

	mw := io.MultiWriter(os.Stdout, logfile)
	return log.New(mw, "", log.LstdFlags)
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
		return config{}, errors.New("config file not found")
	}
	return c, nil
}
