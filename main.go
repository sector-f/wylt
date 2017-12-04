package main

import (
	"fmt"
	"log"
	"math"
	"strconv"
	"time"

	"github.com/fhs/gompd/mpd"
)

type playingStatus struct {
	Track    string
	Duration string
	Elapsed  string
}

func getStatus(c *mpd.Client) playingStatus {
	status, err := c.Status()
	check(err, "status")
	if status["state"] == "pause" {
		return playingStatus{
			Track:    "paused",
			Duration: status["duration"],
			Elapsed:  status["elapsed"]}
	} else if status["state"] == "play" {
		song, err := c.CurrentSong()
		check(err, "current song")
		return playingStatus{
			Track:    song["Title"] + " by " + song["Artist"],
			Duration: status["duration"],
			Elapsed:  status["elapsed"]}
	}
	// mpd is not playing
	return playingStatus{
		Track:    "Nothing",
		Duration: "Nothing",
		Elapsed:  "Nothing"}
}

func keepAlive(c *mpd.Client) {
	go func() {
		err := c.Ping()
		check(err, "ping")
		time.Sleep(30 * time.Second)
		keepAlive(c)
	}()
}

func parseDuration(s playingStatus) (time.Duration, time.Duration) {
	totalLength, err := strconv.ParseFloat(s.Duration, 64)
	check(err, "totalLength")

	tl := time.Duration(int(totalLength)) * time.Second

	hl := time.Duration(int(math.Floor(totalLength/2))) * time.Second

	return tl, hl
}

func main() {
	address := "127.0.0.1:6600"
	// Connect to mpd and create a watcher for its events.
	w, err := mpd.NewWatcher("tcp", address, "")
	check(err, "watcher")
	// Connect to mpd as a client.
	c, err := mpd.Dial("tcp", address)
	check(err, "dial")
	keepAlive(c)

	// Create channel that will keep track of the current playing track.
	currentTrack := make(chan string)

	// get initial track's status
	go func() {
		is := getStatus(c)
		fmt.Println("Current track:", is.Track)
		currentTrack <- is.Track
	}()

	// Log events.
	for subsystem := range w.Event {
		if subsystem == "player" {
			go func() {
				// get old track
				t := <-currentTrack

				// Connect to mpd to get the current track
				s := getStatus(c)
				// check against old one
				if s.Track != t {
					// if it's not the same, start a timer
					fmt.Println("Track changed:", s.Track)
					_, hl := parseDuration(s)
					time.AfterFunc(hl, func() {
						fmt.Println("done!", s.Track)
					})
				}
				go func() {
					currentTrack <- s.Track
				}()
			}()
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
		log.Fatalln("error here: ", where, e)
	}
}
