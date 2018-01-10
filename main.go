package main

import (
	"errors"
	"io/ioutil"
	"log"
	"math"
	"os"
	"strconv"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/fhs/gompd/mpd"
)

// struct for what we use
type playingStatus struct {
	Track    string
	Title    string
	Artist   string
	Album    string
	Duration string
	Elapsed  string
	Status   string
}

type config struct {
	Address string
	Token   string
}

// parse the status and return useful info
func getStatus(c *mpd.Client) (playingStatus, error) {
	status, err := c.Status()
	check(err, "status")
	if status["state"] == "play" || status["state"] == "pause" {
		song, err := c.CurrentSong()
		check(err, "current song")
		return playingStatus{
			Track:    song["Title"] + " by " + song["Artist"],
			Title:    song["Title"],
			Artist:   song["Artist"],
			Album:    song["Album"],
			Duration: status["duration"],
			Elapsed:  status["elapsed"],
			Status:   status["state"]}, nil
	}
	return playingStatus{}, errors.New("MPD is not playing anything")
}

// keep the connection alive
func keepAlive(c *mpd.Client) {
	err := c.Ping()
	check(err, "ping")
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
func startTimer(c *mpd.Client, t string, d string) *time.Timer {
	// get half point of the track's duration
	hp := getHalfPoint(d)
	// check if half point is shorter than 4 minutes
	var td int
	if hp < 240 {
		td = hp
	} else {
		td = 240
	}
	// create timer that lasts for half the duration of the playing track
	// or four minutes, whichever is shorter
	timer := time.AfterFunc(time.Duration(td)*time.Second, func() {
		s, err := getStatus(c)
		check(err, "timer status")
		if t == s.Track {
			callAPI(s)
		}
	})
	return timer
}

// submit finished plays to listenBrainz
func callAPI(s playingStatus) {
	log.Println("API called:", s.Track)
}

func main() {
	// read config file
	path := os.Getenv("XDG_CONFIG_HOME") + "/libra/config.toml"
	configFile, err := ioutil.ReadFile(path)
	check(err, "ReadFile")

	// parse config file
	var conf config
	if _, err := toml.Decode(string(configFile), &conf); err != nil {
		log.Fatalln("Config file not found.")
	}

	a := conf.Address

	// Connect to mpd as a client.
	c, err := mpd.Dial("tcp", a)
	check(err, "dial")

	// Connect to mpd and create a watcher for its events.
	w, err := mpd.NewWatcher("tcp", a, "")
	check(err, "watcher")
	// keep the connection alive
	keepAlive(c)

	// get initial status
	s, err := getStatus(c)
	check(err, "getStatus initial status")
	log.Println("Initial track:", s.Track)

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
			if err == nil {
				log.Println("Playing Now:", s.Track)
				timer := startTimer(c, s.Track, s.Duration)
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
