package main

import (
	"io/ioutil"
	"log"
	"os"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/fhs/gompd/mpd"
	u "github.com/kori/libra/utils"
)

type config struct {
	Address string
	Token   string
}

func getConfig() config {
	// read config file
	path := os.Getenv("XDG_CONFIG_HOME") + "/libra/config.toml"
	configFile, err := ioutil.ReadFile(path)
	check(err, "ReadFile")

	// parse config file
	var conf config
	if _, err := toml.Decode(string(configFile), &conf); err != nil {
		log.Fatalln("Config file not found.")
	}

	return conf
}

type singlePayload struct {
	listenedAt int
	trackMetadata struct {
		artistName string
		trackName string
		releaseName string
	}

// { "listen_type": "single", "payload": [
//     {
//       "listened_at": 1443521965,
//       "track_metadata": {
//         "additional_info": {
//           "release_mbid": "bf9e91ea-8029-4a04-a26a-224e00a83266",
//           "artist_mbids": [
//             "db92a151-1ac2-438b-bc43-b82e149ddd50"
//           ],
//           "recording_mbid": "98255a8c-017a-4bc7-8dd6-1fa36124572b",
//           "tags": [ "you", "just", "got", "rick rolled!"]
//         },
//         "artist_name": "Rick Astley",
//         "track_name": "Never Gonna Give You Up",
//         "release_name": "Whenever you need somebody"
//       }
//     }
//   ]
// }


func main() {
	// get config info
	conf := getConfig()

	// Connect to mpd as a client.
	c, err := mpd.Dial("tcp", conf.Address)
	check(err, "dial")

	// Connect to mpd and create a watcher for its events.
	w, err := mpd.NewWatcher("tcp", conf.Address, "")
	check(err, "watcher")
	// keep the connection alive
	u.KeepAlive(c)

	// get initial status
	s, err := u.GetStatus(c)
	check(err, "GetStatus initial status")
	log.Println("Initial track:", s.Track)

	// create channel that will keep track of the current timer
	var currentTimer = make(chan *time.Timer, 1)

	// watch for subsystem changes
	for subsystem := range w.Event {
		if subsystem == "player" {
			// check if there's a timer running, and if there is, stop it
			if len(currentTimer) > 0 {
				t := <-currentTimer
				t.Stop()
			}
			// Connect to mpd to get the current track
			s, err := u.GetStatus(c)
			// if there's anything playing, log it
			if err == nil {
				log.Println("Playing Now:", s.Track)
				timer := u.StartTimer(c, s.Track, s.Duration)
				// update current timer
				currentTimer <- timer
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
		log.Fatalln("error here:", where, e)
	}
}
