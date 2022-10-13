package main

import (
	"flag"
	"log"
	"math"
	"os"
	"time"

	"github.com/jrudio/go-plex-client"
	"github.com/yanando/lastfm_scrobbler/lastfm"
	"github.com/yanando/lastfm_scrobbler/logger"
)

func main() {
	var serverURL string
	var plexToken string
	// var plexUser string
	var lastFMUser string
	var plexMusicLibrary string
	var verbose bool
	flag.StringVar(&serverURL, "s", "http://localhost:32400", "Plex server url (defaults to http://localhost:32400")
	flag.StringVar(&plexToken, "t", "", "Plex token")
	// flag.StringVar(&plexUser, "pu", "", "Plex user to scrobble from, scrobbles from all users by default")
	flag.StringVar(&lastFMUser, "lu", "", "LastFM user to scrobble from, not required if only 1 user is logged in")
	flag.StringVar(&plexMusicLibrary, "m", "Music", "Title of the plex music library (defaults to Music)")
	flag.BoolVar(&verbose, "v", false, "enable verbose logging")

	flag.Parse()

	logger.Debug = verbose

	if plexToken == "" {
		log.Fatalln("Please supply a plex token")
	}

	lastFM, err := lastfm.FromSessionFile(lastFMUser)

	if err != nil {
		log.Fatalf("Error logging in to lastfm: %s\n", err)
	}

	logger.LogInfo("Logged in!")

	plexConn, err := plex.New(serverURL, plexToken)

	if err != nil {
		log.Fatalf("Error connecting to plex server: %s\n", err)
	}

	exit := make(chan os.Signal, 1)

	events := plex.NewNotificationEvents()

	var lastScrobbled string
	var nowPlaying string
	var started time.Time = time.Now()
	var cachedScrobble *lastfm.Scrobble

	events.OnPlaying(func(n plex.NotificationContainer) {
		metadata, err := plexConn.GetMetadata(n.PlaySessionStateNotification[0].RatingKey)

		if err != nil {
			logger.LogError("error getting metadata: %s", err)
			return
		}

		m := metadata.MediaContainer.Metadata[0]

		if m.LibrarySectionTitle != plexMusicLibrary ||
			n.PlaySessionStateNotification[0].State != "playing" || n.Type != "playing" {
			return
		}

		// reset last scrobbled, new track is being played
		if n.PlaySessionStateNotification[0].ViewOffset < 3000 && lastScrobbled != "" {
			lastScrobbled = ""
			started = time.Now()
			logger.LogDebug("Restarted scrobble session")

			if cachedScrobble != nil {
				err = lastFM.Scrobble(cachedScrobble)

				if err != nil {
					logger.LogError("Error scrobbling track: %s", err)
				}

				logger.LogInfo("%s Scrobbled %s - %s", lastFM.Username, cachedScrobble.Track, cachedScrobble.Album)
				cachedScrobble = nil
			}
		}

		currentSeconds := int(math.Round(float64(n.PlaySessionStateNotification[0].ViewOffset) / 1000))
		durationSeconds := int(math.Round(float64(m.Duration) / 1000))

		scrobble := &lastfm.Scrobble{
			Album:      m.ParentTitle,
			Artist:     m.GrandparentTitle,
			Track:      m.Title,
			Duration:   durationSeconds,
			TrackIndex: int(m.Index),
			StartTime:  started,
		}

		// only scrobble if track is longer than 30 seconds and the track has been played for at least half its duration,
		// or for 4 minutes (whichever occurs earlier.). Cache scrobble and submit when next track is being played to prevent
		// showing the scrobble entry next to the now playing entry
		if durationSeconds > 30 &&
			(float64(currentSeconds) >= float64(durationSeconds)*0.5 || currentSeconds >= 4*60) &&
			lastScrobbled == "" {
			cachedScrobble = scrobble

			lastScrobbled = n.PlaySessionStateNotification[0].RatingKey
			logger.LogDebug("Added %s - %s to scrobble cache, will be scrobbled once next track starts playing", scrobble.Track, scrobble.Album)
		}

		if nowPlaying != n.PlaySessionStateNotification[0].RatingKey {
			err = lastFM.NowPlaying(scrobble)

			if err != nil {
				logger.LogError("error setting nowplaying on track: %s", err)
			}
			nowPlaying = n.PlaySessionStateNotification[0].RatingKey

			logger.LogInfo("%s is now playing %s - %s", lastFM.Username, scrobble.Track, scrobble.Album)
		}
	})

	plexConn.SubscribeToNotifications(events, exit, func(err error) {})

	select {}
}
