package main

import (
	"flag"
	"log"
	"math"
	"os"
	"strings"
	"time"

	"github.com/jrudio/go-plex-client"
	"github.com/yanando/lastfm_scrobbler/lastfm"
	"github.com/yanando/lastfm_scrobbler/logger"
	"gopkg.in/yaml.v2"
)

type config struct {
	ServerURL        string `yaml:"server url"`
	PlexToken        string `yaml:"plex token"`
	LastFMUser       string `yaml:"lastfm user"`
	PlexMusicLibrary string `yaml:"plex music library"`
	Verbose          bool   `yaml:"verbose"`
}

func main() {
	c := config{}
	var configPath string

	flag.StringVar(&c.ServerURL, "s", "http://localhost:32400", "Plex server url")
	flag.StringVar(&c.PlexToken, "t", "", "Plex token")
	flag.StringVar(&c.LastFMUser, "lu", "", "LastFM user to scrobble from, not required if only 1 user is logged in")
	flag.StringVar(&c.PlexMusicLibrary, "m", "Music", "Title of the plex music library")

	flag.StringVar(&configPath, "c", "", "Path to config file")

	flag.BoolVar(&c.Verbose, "v", false, "enable verbose logging")

	flag.Parse()

	if configPath != "" {
		configBytes, err := os.ReadFile(configPath)

		if err != nil {
			log.Fatalf("Error reading config file: %s\n", err)
		}

		err = yaml.Unmarshal(configBytes, &c)

		if err != nil {
			log.Fatalf("Error parsing config file: %s\n", err)
		}
	}

	logger.Debug = c.Verbose

	if c.PlexToken == "" {
		log.Fatalln("Please supply a plex token")
	}

	lastFM, err := lastfm.FromSessionFile(c.LastFMUser)

	if err != nil {
		log.Fatalf("Error logging in to lastfm: %s\n", err)
	}

	logger.LogInfo("Logged in!")

	plexConn, err := plex.New(c.ServerURL, c.PlexToken)

	if err != nil {
		log.Fatalf("Error connecting to plex server: %s\n", err)
	}

	events := plex.NewNotificationEvents()

	var lastScrobbled string
	var started time.Time = time.Now()
	var cachedScrobble *lastfm.Scrobble

	events.OnPlaying(func(n plex.NotificationContainer) {
		metadata, err := plexConn.GetMetadata(n.PlaySessionStateNotification[0].RatingKey)

		if err != nil {
			logger.LogError("error getting metadata: %s", err)
			return
		}

		m := metadata.MediaContainer.Metadata[0]

		if m.LibrarySectionTitle != c.PlexMusicLibrary ||
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

				logger.LogInfo("%s scrobbled %s - %s", lastFM.Username, cachedScrobble.Track, cachedScrobble.Album)
				cachedScrobble = nil
			}
		}

		currentSeconds := int(math.Round(float64(n.PlaySessionStateNotification[0].ViewOffset) / 1000))
		durationSeconds := int(math.Round(float64(m.Duration) / 1000))

		scrobble := &lastfm.Scrobble{
			Album:      m.ParentTitle,
			Artist:     normalizeString(m.GrandparentTitle), // Normalize string to prevent scrobbling the wrong track/artist
			Track:      normalizeString(m.Title),            // Normalize string to prevent scrobbling the wrong track/artist
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

		err = lastFM.NowPlaying(scrobble)

		if err != nil {
			logger.LogError("error setting nowplaying on track: %s", err)
		}

		logger.LogInfo("%s is now playing %s - %s", lastFM.Username, scrobble.Track, scrobble.Artist)
	})

	for {
		subscribe(plexConn, events)
		// subscribetonotifictions encountered an error, try to reinitialize a plexconn and then resubscribe
		logger.LogError("Connection to plex server lost, trying to reconnect in 5 seconds")
		for {
			time.Sleep(time.Second * 5)
			plexConn, err = plex.New(c.ServerURL, c.PlexToken)

			if err != nil {
				logger.LogError("Error re-establishing connection to Plex Server: %v", err)
				continue
			}
			break
		}
	}
}

func normalizeString(input string) string {
	input = strings.ReplaceAll(input, "’", "'")
	input = strings.ReplaceAll(input, "‐", "-")
	return input
}

func subscribe(plexConn *plex.Plex, events *plex.NotificationEvents) {
	exit := make(chan os.Signal, 1)
	wait := make(chan struct{}, 1)
	plexConn.SubscribeToNotifications(events, exit, func(err error) {
		logger.LogDebug("subcribeToNotifications: %v", err)

		logger.LogDebug("got error, trying to re-establish connection")
		exit <- os.Kill
		wait <- struct{}{}
	})
	<-wait
}
