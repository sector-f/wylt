package main

import (
	"fmt"
	"log"
	"math"
	"strconv"
	"time"

	"github.com/fhs/gompd/mpd"
)

func getStatus(c *mpd.Client) (string, string) {
	status, err := c.Status()
	check(err, "status")
	if status["state"] == "pause" {
		return "paused"
	} else if status["state"] == "play" {
		song, err := c.CurrentSong()
		check(err, "current song")
		return song["Title"] + " by " + song["Artist"], status["duration"]
	}
	// mpd is not playing
	return "Nothing", "Nothing"
}

func keepAlive(c *mpd.Client) {
	go func() {
		c.Ping()
		time.Sleep(30 * time.Second)
		keepAlive(c)
	}()
}

func checkDuration(dur string) {
	totalSeconds, err := strconv.ParseFloat(dur, 64)
	check(err, "totalSeconds")

	halftotal := int(math.Floor(totalSeconds / 2))

	go func() {
		time.Sleep(time.Duration(halftotal) * time.Second)

		elapsedSeconds, err := strconv.ParseFloat(status["elapsed"], 64)
		check(err, "eseconds")

		if int(math.Floor(elapsedSeconds)) >= halftotal {
			fmt.Println(fmt.Sprintf("played over half: %s - %s", song["Artist"], song["Title"]))
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
		it, dur := getStatus(c)
		fmt.Println("Current track:", it)
		currentTrack <- it
	}()

	// Log events.
	for subsystem := range w.Event {
		if subsystem == "player" {
			go func() {
				// get old track
				t := <-currentTrack

				// Connect to mpd to get the current track
				ct, dur := getStatus(c)
				// check against old one
				if ct != t {
					// if it's not the same, restart the timer
					fmt.Println("Track changed:", ct)
				} else {
					// if it's the same, keep the timer running
					fmt.Println("Nothing changed")
				}
				go func() {
					currentTrack <- ct
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
