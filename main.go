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
	"sync"

	"github.com/kori/libra/players/mpd"

	"github.com/BurntSushi/toml"
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

	automaticChan, explicitChan, errorChan := mpd.Watch(conf.Address)

	go func() {
		for err := range errorChan {
			log.Println(err)
		}
	}()

	go func() {
		for s := range automaticChan {
			var t = struct {
				Title  string
				Artist string
				Album  string
			}{
				s.Title,
				s.Artist,
				s.Album,
			}

			log.Println("mpd: Now playing:", t.Title, "by", t.Artist, "on", t.Album)
			r, err := lb.SubmitPlayingNow(lb.Track(t), conf.Token)
			if err != nil {
				log.Println(err)
			}
			log.Println("listenbrainz:", r.Status+":", track)
		}
	}()

	var wg sync.WaitGroup
	wg.Add(1)
	wg.Wait()
}
