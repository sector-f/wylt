package utils

import (
	"encoding/json"
	"errors"
	"log"
	"math"
	"strconv"
	"time"

	"github.com/fhs/gompd/mpd"
)

type Submission struct {
	ListenType string    `json:"listen_type"`
	Payloads   []Payload `json:"payload"`
}

type Payloads []Payload

type Payload struct {
	ListenedAt    int `json:"listened_at"`
	TrackMetadata `json:"track_metadata"`
}

type TrackMetadata struct {
	ArtistName  string `json:"artist_name"`
	TrackName   string `json:"track_name"`
	ReleaseName string `json:"release_name"`
}

// submit finished plays to listenBrainz
func CallAPI(s PlayingStatus) {
	p := Submission{
		ListenType: "single",
		Payloads: Payloads{
			Payload: Payload{
				ListenedAt: int(time.Now().Unix()),
				TrackMetadata: TrackMetadata{
					s.Title,
					s.Artist,
					s.Album,
				}},
		}}
	pm, _ := json.Marshal(p)
	log.Println("API called:", string(pm))
}

// struct for what we use
type PlayingStatus struct {
	Track    string
	Title    string
	Artist   string
	Album    string
	Duration string
	Elapsed  string
	Status   string
}

// parse the status and return useful info
func GetStatus(c *mpd.Client) (PlayingStatus, error) {
	status, err := c.Status()
	check(err, "status")
	if status["state"] == "play" || status["state"] == "pause" {
		song, err := c.CurrentSong()
		check(err, "current song")
		return PlayingStatus{
			Track:    song["Title"] + " by " + song["Artist"],
			Title:    song["Title"],
			Artist:   song["Artist"],
			Album:    song["Album"],
			Duration: status["duration"],
			Elapsed:  status["elapsed"],
			Status:   status["state"]}, nil
	}
	return PlayingStatus{}, errors.New("MPD is not playing anything")
}

// keep the connection alive
func KeepAlive(c *mpd.Client) {
	err := c.Ping()
	check(err, "ping")
	go func() {
		time.Sleep(30 * time.Second)
		KeepAlive(c)
	}()
}

// get the mid point of a track's duration
func getHalfPoint(d string) int {
	totalLength, err := strconv.ParseFloat(d, 64)
	check(err, "totalLength")

	return int(math.Floor(totalLength / 2))
}

// start timer for the current playing track
func StartTimer(c *mpd.Client, t string, d string) *time.Timer {
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
		s, err := GetStatus(c)
		check(err, "timer status")
		if t == s.Track {
			CallAPI(s)
		}
	})
	return timer
}

func check(e error, where string) {
	if e != nil {
		log.Fatalln("error here:", where, e)
	}
}
