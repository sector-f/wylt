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

	"github.com/BurntSushi/toml"

	p "github.com/kori/libra/players"
	"github.com/kori/libra/players/mpd"

	"github.com/kori/libra/services/listenbrainz"
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

	// create channels to keep track of the current statuses
	var statusChan = make(chan p.Status)
	var errorChan = make(chan error)

	// start mpd watcher
	go mpd.Watch(conf.Address, statusChan, errorChan)

	for s := range statusChan {
		track := s.Title + " by " + s.Artist
		log.Println("mpd: Now playing:", track)

		r, err := listenbrainz.PostPlayingNow(s, conf.Token)
		if err != nil {
			log.Println(err)
		}
		log.Println("listenbrainz:", r.Status+":", track)
	}

	go func() {
		for s := range errorChan {
			log.Println(s)
		}
	}()
}
