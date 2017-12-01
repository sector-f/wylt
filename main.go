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
		s := playingStatus{
			Track:    "paused",
			Duration: status["duration"],
			Elapsed:  status["elapsed"]}
		return s
	} else if status["state"] == "play" {
		song, err := c.CurrentSong()
		check(err, "current song")
		s := playingStatus{
			Track:    song["Title"] + " by " + song["Artist"],
			Duration: status["duration"],
			Elapsed:  status["elapsed"]}
		return s
	}
	// mpd is not playing
	n := playingStatus{"", "", ""}
	return n
}

func keepAlive(c *mpd.Client) {
	go func() {
		c.Ping()
		time.Sleep(30 * time.Second)
		keepAlive(c)
	}()
}

func checkDuration(s playingStatus) {
	totalSeconds, err := strconv.ParseFloat(s.Duration, 64)
	check(err, "totalSeconds")

	halftotal := int(math.Floor(totalSeconds / 2))

	go func() {
		time.Sleep(time.Duration(halftotal) * time.Second)

		elapsedSeconds, err := strconv.ParseFloat(s.Elapsed, 64)
		check(err, "eseconds")

		if int(math.Floor(elapsedSeconds)) >= halftotal {
			fmt.Println("played over half: ", s.Track)
		}
	}()
}

func main() {
	// Connect to mpd and create a watcher for its events.
	w, err := mpd.NewWatcher("tcp", "127.0.0.1:6600", "")
	check(err, "watcher")
	defer w.Close()
	// Connect to mpd as a client.
	c, err := mpd.Dial("tcp", "127.0.0.1:6600")
	check(err, "dial")
	keepAlive(c)
	defer c.Close()

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
					// if it's not the same, restart the timer
					fmt.Println("Track changed:", s.Track)
				} else {
					// if it's the same, keep the timer running
					fmt.Println("Nothing changed")
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
}

func check(e error, where string) {
	if e != nil {
		log.Fatalln("error here: ", where, e)
	}
}
