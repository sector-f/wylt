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
	"io/ioutil"
	"log"
	"os"
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
func getConfig() config {
	// read config file
	path := os.Getenv("XDG_CONFIG_HOME") + "/libra/config.toml"
	configFile, err := ioutil.ReadFile(path)
	if err != nil {
		log.Fatalln(err)
	}

	// parse config file and assign to a struct
	var c config
	if _, err := toml.Decode(string(configFile), &c); err != nil {
		log.Fatalln("Config file not found.")
	}
	return c
}

func main() {
	// get config info from $PATH
	conf := getConfig()

	// set up channels for automatic monitoring , explicit requests, and errors
	mpdEvents, playingNow, errors := mpd.Watch(conf.Address)

	// watch the errors channel
	go func() {
		for err := range errors {
			log.Println(err)
		}
	}()

	// watch the automatic events channel
	for current := range mpdEvents {
		func(old p.Status) {
			log.Println("mpd: Playing now:", current.Title, "by", current.Artist, "on", current.Album)

			r, err := lb.SubmitPlayingNow(lb.Track(current.Track), conf.Token)
			if err != nil {
				log.Println(err)
			}
			log.Println("listenbrainz:", r.Status+":", "Playing now:", current.Track)
			r, err := lb.SubmitPlayingNow(lb.Track(current.Track), conf.Token)
			if err != nil {
				log.Println(err)
			}
			log.Println("listenbrainz:", r.Status+":", "Playing now:", current.Track)

			// submit the track if the submission time has elapsed and if it's still the same track
			time.AfterFunc(time.Duration(lb.GetSubmissionTime(current.Duration))*time.Second, func() {
				new := <-playingNow
				if old.Track == new.Track {
					r, err := lb.SubmitSingle(lb.Track(old.Track), conf.Token)
					if err != nil {
						log.Println(err)
					}
					log.Println("listenbrainz:", r.Status+":", "Single submission:", old.Track)
				}
			})
		}(current)
	}
}
