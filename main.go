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
	"bytes"
	"encoding/json"
	"errors"
	"io/ioutil"
	"log"
	"math"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/fhs/gompd/mpd"
)

// Submission is a struct for marshalling the JSON payload
type Submission struct {
	ListenType string    `json:"listen_type"`
	Payloads   []Payload `json:"payload"`
}

// Payloads is a helper struct for marshalling the JSON payload
type Payloads []Payload

// Payload is a helper struct for marshalling the JSON payload
type Payload struct {
	ListenedAt    int `json:"listened_at,omitempty"`
	TrackMetadata `json:"track_metadata"`
}

// TrackMetadata is a helper struct for marshalling the JSON payload
type TrackMetadata struct {
	TrackName   string `json:"track_name"`
	ArtistName  string `json:"artist_name"`
	ReleaseName string `json:"release_name"`
}

// struct for unmarshalling the config
type config struct {
	Address string
	Token   string
}

// struct for encoding the current status of the player
type playingStatus struct {
	Track    string
	Title    string
	Artist   string
	Album    string
	Duration string
	Elapsed  string
	State    string
}

// read configuration file and return a config struct
func getConfig() config {
	// read config file
	path := os.Getenv("XDG_CONFIG_HOME") + "/libra/config.toml"
	configFile, err := ioutil.ReadFile(path)
	check(err, "ReadFile@getConfig")

	// parse config file and assign to a struct
	var c config
	if _, err := toml.Decode(string(configFile), &c); err != nil {
		log.Fatalln("Config file not found.")
	}

	return c
}

// keep the connection alive
func keepAlive(c *mpd.Client) {
	err := c.Ping()
	check(err, "ping")

	// call keepAlive again after 30 seconds
	go func() {
		time.Sleep(30 * time.Second)
		keepAlive(c)
	}()
}

// query mpd for the status of the player
func getStatus(c *mpd.Client) (playingStatus, error) {
	status, err := c.Status()
	check(err, "status")

	if status["state"] == "play" || status["state"] == "pause" {
		// query mpd for the info about the current song
		song, err := c.CurrentSong()
		check(err, "CurrentSong@getStatus")

		// return struct with the info libra uses the most
		return playingStatus{
			Track:    song["Title"] + " by " + song["Artist"],
			Title:    song["Title"],
			Artist:   song["Artist"],
			Album:    song["Album"],
			Duration: status["duration"],
			Elapsed:  status["elapsed"],
			State:    status["state"]}, nil
	}

	return playingStatus{}, errors.New("MPD is not playing anything")
}

// format listen info as JSON
func formatJSON(s playingStatus, lt string) []byte {
	var p Payload
	// insert values into struct
	if lt == "playing_now" {
		p = Payload{
			TrackMetadata: TrackMetadata{
				s.Title,
				s.Artist,
				s.Album,
			}}
	} else if lt == "single" {
		p = Payload{
			ListenedAt: int(time.Now().Unix()),
			TrackMetadata: TrackMetadata{
				s.Title,
				s.Artist,
				s.Album,
			}}

	} else {
		return []byte("")
	}

	sp := Submission{
		ListenType: lt,
		Payloads: Payloads{
			p,
		},
	}

	// convert struct to JSON and return it
	pm, err := json.Marshal(sp)
	check(err, "Marshal@formatJSON")
	return pm
}

// start timer for the current playing track
func startTimer(c *mpd.Client, s playingStatus, conf config) *time.Timer {
	// get half point of the track's duration
	totalLength, err := strconv.ParseFloat(s.Duration, 64)
	check(err, "totalLength")
	hp := int(math.Floor(totalLength / 2))

	// source: https://listenbrainz.readthedocs.io/en/latest/dev/api.html
	// Listens should be submitted for tracks when the user has listened to
	// half the track or 4 minutes of the track, whichever is lower. If the
	// user hasn’t listened to 4 minutes or half the track, it doesn’t fully
	// count as a listen and should not be submitted.
	var td int
	if hp > 240 {
		td = 240
	} else {
		td = hp
	}

	// create timer
	timer := time.AfterFunc(time.Duration(td)*time.Second, func() {
		cs, err := getStatus(c)
		check(err, "timer status")

		if s.Track == cs.Track {
			submitRequest(formatJSON(cs, "single"), conf)
			log.Println("Track submitted:", cs.Track)
		}
	})

	return timer
}

// submit the given status to ListenBrainz
func submitRequest(j []byte, conf config) *http.Response {
	url := "https://api.listenbrainz.org/1/submit-listens"

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(j))
	check(err, "NewRequest@submitRequest")
	req.Header.Set("Authorization", "Token "+conf.Token)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	check(err, "Do@submitRequest")

	defer resp.Body.Close()
	return resp
}

func main() {
	// get config info
	conf := getConfig()

	// Connect to mpd as a client.
	c, err := mpd.Dial("tcp", conf.Address)
	check(err, "dial")
	// keep the connection alive
	keepAlive(c)

	// Connect to mpd and create a watcher for its events.
	w, err := mpd.NewWatcher("tcp", conf.Address, "")
	check(err, "watcher")

	// get initial status
	s, err := getStatus(c)
	if err != nil {
		log.Println(err)
	} else {
		log.Println("Initial track:", s.Track)
		response := submitRequest(formatJSON(s, "playing_now"), conf)
		log.Println("response status:", s.Track, ":", response.Status)
	}

	// create channel that will keep track of the current timer
	var currentTimer = make(chan *time.Timer, 1)

	// watch for subsystem changes
	for subsystem := range w.Event {
		if subsystem == "player" {
			// check if there's a timer running, and if there is, stop it
			if len(currentTimer) > 0 {
				t := <-currentTimer
				t.Stop()
			}
			// Connect to mpd to get the current track
			s, err := getStatus(c)
			// if there's anything playing, log it
			if err == nil && s.State == "play" {
				log.Println("Playing Now:", s.Track)
				response := submitRequest(formatJSON(s, "playing_now"), conf)
				log.Println("response status:", s.Track, ":", response.Status)
				timer := startTimer(c, s, conf)
				// update current timer
				currentTimer <- timer
			}
		}
	}
	// Log errors.
	go func() {
		for err := range w.Error {
			log.Println("Error:", err)
		}
	}()

	// Clean everything up.
	err = c.Close()
	check(err, "client close")
	err = w.Close()
	check(err, "watcher close")
}

func check(e error, where string) {
	if e != nil {
		log.Fatalln("error here:", where, e)
	}
}
