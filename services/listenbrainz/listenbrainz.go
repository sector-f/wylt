package listenbrainz

import (
	"bytes"
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"strconv"
	"time"

	p "github.com/kori/libra/players"
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

// format listen info as JSON for ListenBrainz
func formatAsJSON(s p.Status, lt string) ([]byte, error) {
	// these values don't change
	tm := TrackMetadata{
		s.Title,
		s.Artist,
		s.Album,
	}

	// insert values into struct
	var p Payload
	if lt == "playing_now" {
		p = Payload{TrackMetadata: tm}
	} else if lt == "single" {
		p = Payload{
			ListenedAt:    int(time.Now().Unix()),
			TrackMetadata: tm,
		}
	} else {
		// there's nothing to return
		return nil, errors.New("Unrecognized listen type.")
	}

	sp := Submission{
		ListenType: lt,
		Payloads: Payloads{
			p,
		},
	}

	// convert struct to JSON and return it
	pm, err := json.Marshal(sp)
	if err != nil {
		return nil, err
	}
	return pm, nil
}

func getSubmissionTime(d string) (int, error) {
	// get halfway point of the track's duration
	totalLength, err := strconv.ParseFloat(d, 64)
	if err != nil {
		return 0, err
	}
	hp := int(math.Floor(totalLength / 2))

	// source: https://listenbrainz.readthedocs.io/en/latest/dev/api.html
	// Listens should be submitted for tracks when the user has listened to
	// half the track or 4 minutes of the track, whichever is lower. If the
	// user hasn’t listened to 4 minutes or half the track, it doesn’t fully
	// count as a listen and should not be submitted.
	var st int
	if hp > 240 {
		st = 240
	} else {
		st = hp
	}
	return st, nil
}

// create a request with the given json info
func createRequest(json []byte, token string) (*http.Request, error) {
	url := "https://api.listenbrainz.org/1/submit-listens"

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(json))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Token "+token)
	req.Header.Set("Content-Type", "application/json")

	return req, nil
}

// SubmitRequest executes the request
func SubmitRequest(req *http.Request) (*http.Response, error) {
	client := &http.Client{}

	resp, err := client.Do(req)
	defer resp.Body.Close()

	if err != nil {
		return nil, err
	}

	return resp, nil
}

// PostPlayingNow posts the given Status to ListenBrainz as what's playing now.
func PostPlayingNow(s p.Status, token string) (*http.Response, error) {
	j, err := formatAsJSON(s, "playing_now")
	if err != nil {
		return nil, err
	}

	req, err := createRequest(j, token)
	if err != nil {
		return nil, err
	}

	response := SubmitRequest(req)
	return response, nil
}

// PostSingle posts the given Status to ListenBrainz as a single listen.
func PostSingle(s p.Status, token string) (*http.Response, error) {
	j, err := formatAsJSON(s, "single")
	if err != nil {
		return nil, err
	}

	req, err := createRequest(j, token)
	if err != nil {
		return nil, err
	}

	response := SubmitRequest(req)
	return response, nil
}
