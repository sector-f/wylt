package main

import (
	"errors"
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
	Status   string
}

func getStatus(c *mpd.Client) (playingStatus, error) {
	status, err := c.Status()
	check(err, "status")
	if status["state"] == "play" || status["state"] == "pause" {
		song, err := c.CurrentSong()
		check(err, "current song")
		return playingStatus{
			Track:    song["Title"] + " by " + song["Artist"],
			Duration: status["duration"],
			Elapsed:  status["elapsed"],
			Status:   status["state"]}, nil
	}
	return playingStatus{}, errors.New("MPD is not playing anything.")
}

func keepAlive(c *mpd.Client) {
	go func() {
		err := c.Ping()
		check(err, "ping")
		time.Sleep(30 * time.Second)
		keepAlive(c)
	}()
}

func getHalfPoint(s playingStatus) int {
	totalLength, err := strconv.ParseFloat(s.Duration, 64)

	check(err, "totalLength")
	return int(math.Floor(totalLength / 2))
}

func main() {
	address := "192.168.1.100:6600"
	// Connect to mpd and create a watcher for its events.
	w, err := mpd.NewWatcher("tcp", address, "")
	check(err, "watcher")
	// Connect to mpd as a client.
	c, err := mpd.Dial("tcp", address)
	check(err, "dial")
	keepAlive(c)

	// get initial status
	is, err := getStatus(c)
	check(err, "getStatus initial status")
	fmt.Println("Current track:", is.Track)

	var oldTrack = make(chan string)
	go func() {
		oldTrack <- is.Track
	}()

	// Watch for track changes.
	for subsystem := range w.Event {
		if subsystem == "player" {
			// Connect to mpd to get the current track
			s, err := getStatus(c)
			if err == nil {
				currentTrack := s.Track
				// add new track
				hp := getHalfPoint(s)
				log.Println("Track changed:", currentTrack)
				time.AfterFunc(time.Duration(hp)*time.Second, func() {
					go func() {
						if currentTrack == <-oldTrack {
							log.Println("API called:", currentTrack)
						}
					}()
				})
				go func() {
					oldTrack <- currentTrack
				}()
			} else {
				log.Println("Playlist cleared.")
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
		log.Println("error here: ", where, e)
	}
}
