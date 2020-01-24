package main

import (
	"net/http"
	"time"

	lb "github.com/kori/go-listenbrainz"
)

// This contains information about the listenbrainz target.
type listenbrainz struct {
	Token string
}

// SubmitPlayingNow wraps a target's "playing now" function. (It's used in last.fm, libre.fm, and listenbrainz.)
func (session *listenbrainz) SubmitPlayingNow(t Track) (*http.Response, error) {
	return lb.SubmitPlayingNow(lb.Track(t), session.Token)
}

// SubmitListen says you've listened to a track, according to a Target's parameters on what counts as a listen.
func (session *listenbrainz) SubmitListen(t Track) (*http.Response, error) {
	return lb.SubmitSingle(lb.Track(t), session.Token, time.Now().Unix())
}

func (session *listenbrainz) GetSubmissionTime(duration int) (int, error) {
	return lb.GetSubmissionTime(duration)
}
