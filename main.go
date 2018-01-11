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

// struct for marshalling the JSON payload
type Submission struct {
	ListenType string    `json:"listen_type"`
	Payloads   []Payload `json:"payload"`
}

type Payloads []Payload

type Payload struct {
	ListenedAt    int `json:"listened_at,omitempty"`
	TrackMetadata `json:"track_metadata"`
}

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
	check(err, "ReadFile")

	// parse config file and assign to a struct
	var c config
	if _, err := toml.Decode(string(configFile), &c); err != nil {
		log.Fatalln("Config file not found.")
	}

	return c
}

// format listen info as JSON
func formatJSON(s playingStatus, lt string) []byte {
	// insert values into struct
	if lt == "playing_now" {
		p := Submission{
			ListenType: lt,
			Payloads: Payloads{
				Payload{
					TrackMetadata: TrackMetadata{
						s.Title,
						s.Artist,
						s.Album,
					}}}}

		// convert struct to JSON and return it
		pm, _ := json.Marshal(p)
		return pm
	} else if lt == "single" {
		p := Submission{
			ListenType: lt,
			Payloads: Payloads{
				Payload{
					ListenedAt: int(time.Now().Unix()),
					TrackMetadata: TrackMetadata{
						s.Title,
						s.Artist,
						s.Album,
					}}}}

		// convert struct to JSON and return it
		pm, _ := json.Marshal(p)
		return pm
	}
	return []byte("")
}

// send play to ListenBrainz
func submitListen(j []byte, token string) *http.Response {
	url := "https://api.listenbrainz.org/1/submit-listens"

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(j))
	check(err, "NewRequest")
	req.Header.Set("Authorization", "Token "+token)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	check(err, "request")

	defer resp.Body.Close()
	return resp
}

// organize information about the player and the track that's currently playing
func getStatus(c *mpd.Client) (playingStatus, error) {
	// query mpd for the status of the player
	status, err := c.Status()
	check(err, "status")

	if status["state"] == "play" || status["state"] == "pause" {
		// query mpd for the info about the current song
		song, err := c.CurrentSong()
		check(err, "current song")

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

// get the mid point of a track's duration
func getHalfPoint(d string) int {
	totalLength, err := strconv.ParseFloat(d, 64)
	check(err, "totalLength")

	return int(math.Floor(totalLength / 2))
}

// start timer for the current playing track
func startTimer(c *mpd.Client, s playingStatus, token string) *time.Timer {
	// get half point of the track's duration
	hp := getHalfPoint(s.Duration)

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
			submitListen(formatJSON(cs, "single"), token)
		}
	})

	return timer
}

func main() {
	// get config info
	conf := getConfig()

	// Connect to mpd as a client.
	c, err := mpd.Dial("tcp", conf.Address)
	check(err, "dial")

	// Connect to mpd and create a watcher for its events.
	w, err := mpd.NewWatcher("tcp", conf.Address, "")
	check(err, "watcher")

	// keep the connection alive
	keepAlive(c)

	// get initial status
	s, err := getStatus(c)
	if err != nil {
		log.Println(err)
	} else {
		log.Println("Initial track:", s.Track, s.Album)
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
				response := submitListen(formatJSON(s, "playing_now"), conf.Token)
				log.Println("response status:", s.Track, ":", response.Status)
				timer := startTimer(c, s, conf.Token)
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
	err = w.Close()
	check(err, "watcher close")
	err = c.Close()
	check(err, "client close")
}

func check(e error, where string) {
	if e != nil {
		log.Fatalln("error here:", where, e)
	}
}
