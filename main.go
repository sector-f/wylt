// Copyright (c) 2023 Luiz de Milon (kori)

// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.

// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.

// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package main

import (
	"context"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/BurntSushi/toml"
)

// struct for unmarshalling the config
type config struct {
	MPDAddress        string
	MPDPassword       string
	ListenbrainzToken string
}

// function to create a logger to both stdout and a *log.Logger
func newLogger(path string) *log.Logger {
	// open the logfile for appending or create it if it doesnt exist
	logfile, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatalln(err)
	}

	mw := io.MultiWriter(os.Stdout, logfile)
	return log.New(mw, "", log.LstdFlags)
}

func main() {
	// Set config root according to XDG standards.
	configRoot := filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "wylt")
	if configRoot == "" {
		configRoot = filepath.Join(os.Getenv("HOME"), ".config", "wylt")
	}

	configPath := filepath.Join(configRoot, "config.toml")

	var conf config
	_, err := toml.DecodeFile(configPath, &conf)
	if err != nil {
		log.Fatalln(err)
	}

	logPath := filepath.Join(configRoot, "logs", "wylt.log")
	logger := newLogger(logPath)
	if err != nil {
		logger.Fatalln(err)
	}

	mpdSession, err := NewMPD(conf.MPDAddress, conf.MPDPassword)
	if err != nil {
		logger.Fatalln(err)
	}

	ts := Targets{&listenbrainz{Token: conf.ListenbrainzToken}}
	ps := Players{&mpdSession}

	var wg sync.WaitGroup

	// For each player...
	for _, p := range ps {
		// And for each target...
		for _, t := range ts {
			wg.Add(1)

			// ...create a function that will watch the player and submit it to the target
			go func(p Player, t Target) {
				defer wg.Done()

				playerLog, errs := p.Subscribe()

				var (
					ctx        context.Context
					cancelFunc func()

					current Track
				)

				for {
					select {
					case new := <-playerLog:
						// TODO: see if there's a better way to uniquely identify tracks
						// e.g. some sort of ID number, or maybe path on the filesystem
						if new == current {
							continue
						}

						if cancelFunc != nil {
							cancelFunc()
						}
						ctx, cancelFunc = context.WithCancel(context.Background())

						logger.Printf("[PLAYER]: now playing: %s by %s\n", new.Title, new.Artist)
						t.SubmitPlayingNow(new)
						go handleTrack(ctx, t, new, logger)

						current = new
					case e := <-errs:
						// player errors are bound to fill up, should the connection be lost.
						// TODO: find a better way to deal with player error logs
						logger.Println(e)
					}
				}
			}(p, t)
		}
	}

	wg.Wait()
}

func handleTrack(ctx context.Context, target Target, track Track, logger *log.Logger) {
	uploadDuration, err := target.GetSubmissionTime(track.Duration)
	if err != nil {
		logger.Println(err)
		return
	}

	timer := time.NewTimer(uploadDuration)
	select {
	case <-timer.C:
		logger.Printf("[TARGET]: %s: submitting: %s by %s\n", target.Name(), track.Title, track.Artist)

		_, err = target.SubmitListen(track)
		if err != nil {
			logger.Println(err)
		}
	case <-ctx.Done():
		timer.Stop()
	}
}
