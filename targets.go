package main

import (
	"net/http"
	"time"

	lb "github.com/kori/go-listenbrainz"
)

// Target is an interface that encodes what works as a target.
// In this case, it can be anything that has a Publish() function that returns a http response.
type Target interface {
	// this prints the name of the target, for logging purposes
	Name() string
	// SubmitPlayingNow wraps a target's "playing now" function. (It's used in last.fm, libre.fm, and listenbrainz.)
	SubmitPlayingNow(Track) (*http.Response, error)
	// SubmitListen says you've listened to a track, according to a Target's parameters on what counts as a listen.
	SubmitListen(Track) (*http.Response, error)
	// GetSubmissionTime says when a listen should be submitted.
	GetSubmissionTime(time.Duration) (time.Duration, error)
}

type Targets []Target

type listenbrainz struct {
	Token string
}

func (target *listenbrainz) Name() string {
	return "listenbrainz"
}

func (target *listenbrainz) SubmitPlayingNow(t Track) (*http.Response, error) {
	lbt := lb.Track{
		Artist: t.Artist,
		Album:  t.Album,
		Title:  t.Title,
	}
	return lb.SubmitPlayingNow(lb.Track(lbt), target.Token)
}

func (target *listenbrainz) SubmitListen(t Track) (*http.Response, error) {
	lbt := lb.Track{
		Artist: t.Artist,
		Album:  t.Album,
		Title:  t.Title,
	}
	return lb.SubmitSingle(lb.Track(lbt), target.Token, time.Now().Unix())
}

func (session *listenbrainz) GetSubmissionTime(d time.Duration) (time.Duration, error) {
	return lb.GetSubmissionTime(d)
}
