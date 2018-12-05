// Copyright (c) 2018 Luiz de Milon (kori)

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
	"errors"
	"io"
	"io/ioutil"
	"log"
	"os"
	"strconv"
	"time"

	p "github.com/kori/libra/players"
	"github.com/kori/libra/players/mpd"

	"github.com/BurntSushi/toml"
	lb "github.com/kori/go-listenbrainz"
)

// struct for unmarshalling the config
type config struct {
	Address string
	Token   string
}

// read configuration file and return a config struct
func getConfig(path string) (config, error) {
	// read config file
	configFile, err := ioutil.ReadFile(path)
	if err != nil {
		return config{}, err
	}

	// parse config file and assign to a struct
	var c config
	_, err = toml.Decode(string(configFile), &c)
	if err != nil {
		return config{}, errors.New("Config file not found.")
	}
	return c, nil
}

// create a logger that will log both to stdout and the given file
func createLogger(logroot string, name string) (*log.Logger, error) {
	mlLogfile, err := os.Create(logroot + name)
	if err != nil {
		return nil, err
	}

	mw := io.MultiWriter(os.Stdout, mlLogfile)
	return log.New(mw, "", log.LstdFlags), nil
}

func postMPDToListenBrainz(current p.Status, playingNow chan p.Status, conf config, l *log.Logger) {
	track := current.Title + " by " + current.Artist + " on " + current.Album
	l.Println("mpd: Playing now:", track)

	// post current track to listenbrainz
	r, err := lb.SubmitPlayingNow(lb.Track(current.Track), conf.Token)
	if err != nil {
		l.Fatalln(err)
	}
	l.Println("listenbrainz:", r.Status+":", "Playing now:", track)

	// submit the track if the submission time has elapsed and if it's still the same track
	time.AfterFunc(time.Duration(lb.GetSubmissionTime(current.Duration))*time.Second, func() {
		new := <-playingNow
		if current.Track == new.Track {
			r, err := lb.SubmitSingle(lb.Track(current.Track), conf.Token, time.Now().Unix())
			if err != nil {
				l.Fatalln(err)
			}
			l.Println("listenbrainz:", r.Status+":", "Single submission:", track)
		}
	})
}

func main() {
	configroot := os.Getenv("XDG_CONFIG_HOME") + "/libra/"
	logroot := os.Getenv("XDG_CONFIG_HOME") + "/libra/log/"

	// get config info from the path
	conf, err := getConfig(configroot + "config.toml")
	if err != nil {
		log.Fatalln(err)
	}

	// set up channels for automatic monitoring, explicit requests, and errors
	mpdEvents, playingNow, errors, err := mpd.Watch(conf.Address)
	if err != nil {
		log.Fatalln(err)
	}

	// Create logger for mpd to listenbrainz events.
	mlLogger, err := createLogger(logroot, "mpd-listenbrainz-"+strconv.FormatInt(time.Now().Unix(), 10))
	if err != nil {
		log.Fatalln(err)
	}

	// watch the errors channel
	go func() {
		for err := range errors {
			mlLogger.Println(err)
		}
	}()

	// get initial status.
	initial := <-playingNow
	postMPDToListenBrainz(initial, playingNow, conf, mlLogger)

	// watch the automatic events channel
	for e := range mpdEvents {
		func(current p.Status) {
			postMPDToListenBrainz(current, playingNow, conf, mlLogger)
		}(e)
	}
}
