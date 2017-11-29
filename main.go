package main

import (
	"fmt"
	"log"

	"github.com/fhs/gompd/mpd"
)

func getStatus(c *mpd.Client) string {
	status, err := c.Status()
	check(err, "status")
	if status["state"] == "pause" {
		return "paused"
	} else if status["state"] == "play" {
		song, err := c.CurrentSong()
		check(err, "current song")
		return song["Title"] + " by " + song["Artist"]
	}
	// mpd is not playing
	return "Nothing"
}

func main() {
	// Connect to mpd and create a watcher for its events.
	w, err := mpd.NewWatcher("tcp", "127.0.0.1:6600", "")
	check(err, "watcher")
	defer w.Close()

	// Create channel that will keep track of the current playing track.
	currentTrack := make(chan string)
	// Connect to mpd as a client.
	go func() {
		c, err := mpd.Dial("tcp", "127.0.0.1:6600")
		check(err, "dial")
		// get initial track
		it := getStatus(c)
		c.Close()
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
				c, err := mpd.Dial("tcp", "127.0.0.1:6600")
				check(err, "dial")
				ct := getStatus(c)
				c.Close()
				// check against old one
				if ct == t {
					// if it's the same, keep the timer running
					fmt.Println("Nothing changed")
				} else {
					// if its the same, restart the timer
					fmt.Println("Track changed:", ct)
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
