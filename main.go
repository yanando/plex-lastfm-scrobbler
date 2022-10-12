package main

import (
	"flag"
	"fmt"
	"log"
	"math"
	"os"
	"time"

	"github.com/jrudio/go-plex-client"
	"github.com/yanando/lastfm_scrobbler/lastfm"
)

func main() {
	var serverURL string
	var plexToken string
	var plexUser string
	var lastFMUser string
	var plexMusicLibrary string
	flag.StringVar(&serverURL, "s", "http://localhost:32400", "Plex server url (defaults to http://localhost:32400")
	flag.StringVar(&plexToken, "t", "", "Plex token")
	flag.StringVar(&plexUser, "pu", "", "Plex user to scrobble from, scrobbles from all users by default")
	flag.StringVar(&lastFMUser, "lu", "", "LastFM user to scrobble from, not required if only 1 user is logged in")
	flag.StringVar(&plexMusicLibrary, "m", "Music", "Title of the plex music library (defaults to Music)")

	flag.Parse()

	if plexToken == "" {
		log.Fatalln("Please supply a plex token")
	}

	lastFM, err := lastfm.FromSessionFile(lastFMUser)

	if err != nil {
		log.Fatalf("Error logging in to lastfm: %s\n", err)
	}

	_ = lastFM
	fmt.Println("logged in")

	plexConn, err := plex.New(serverURL, plexToken)

	if err != nil {
		panic(err)
	}

	exit := make(chan os.Signal, 1)

	events := plex.NewNotificationEvents()

	var lastScrobbled string
	var started time.Time

	events.OnPlaying(func(n plex.NotificationContainer) {
		metadata, err := plexConn.GetMetadata(n.PlaySessionStateNotification[0].RatingKey)

		if err != nil {
			log.Printf("error getting metadata: %s\n", err)
			return
		}

		m := metadata.MediaContainer.Metadata[0]

		if m.LibrarySectionTitle != plexMusicLibrary || n.PlaySessionStateNotification[0].State != "playing" || n.Type != "playing" {
			return
		}

		// reset last scrobbled, new track is being played
		if n.PlaySessionStateNotification[0].ViewOffset == 0 {
			lastScrobbled = ""
			started = time.Now()
		}

		currentSeconds := int(math.Round(float64(n.PlaySessionStateNotification[0].ViewOffset) / 1000))
		durationSeconds := int(math.Round(float64(m.Duration) / 1000))

		// only scrobble if track is longer than 30 seconds and the track has been played for at least half its duration, or for 4 minutes (whichever occurs earlier.)
		if durationSeconds > 30 && (float64(currentSeconds) >= float64(durationSeconds)*0.5 || currentSeconds >= 4*60) && lastScrobbled == "" {
			lastFM.Scrobble(m.ParentTitle, m.GrandparentTitle, m.Title, durationSeconds, int(m.Index), started)
			lastScrobbled = n.PlaySessionStateNotification[0].RatingKey
		} else {
			fmt.Printf("not scrobbling: currentseconds %d, duration/2 %.02f\n", currentSeconds, float64(durationSeconds)/2)
		}

		// fmt.Printf("Title: %s\nArtist: %s\nAlbum: %s\nEvent: %s\n", m.Title, m.GrandparentTitle, m.ParentTitle, n.PlaySessionStateNotification[0].State)
		err = lastFM.NowPlaying(m.ParentTitle, m.GrandparentTitle, m.Title, durationSeconds, int(m.Index))

		if err != nil {
			log.Printf("error setting nowplaying on track: %s\n", err)
		}
	})

	plexConn.SubscribeToNotifications(events, exit, nil)

	select {}
}
