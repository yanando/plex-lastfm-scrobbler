package lastfm

import (
	"crypto/md5"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/yanando/lastfm_scrobbler/logger"
)

var (
	ApiKey       = "b8edfdf74bc415dd85755c09a0923d3f"
	SharedSecret = "2a09de6493dbf45eb11f0db2c7c97848"
)

type Scrobble struct {
	Album      string
	Artist     string
	Track      string
	Duration   int
	TrackIndex int
	StartTime  time.Time
}

type lastFM struct {
	SessionToken string
	Username     string
}

// Nowplaying, duration in seconds
func (lfm *lastFM) NowPlaying(scrobble *Scrobble) error {
	params := map[string]string{
		"album":       scrobble.Album,
		"artist":      scrobble.Artist,
		"api_key":     ApiKey,
		"method":      "track.updateNowPlaying",
		"duration":    strconv.Itoa(scrobble.Duration),
		"sk":          lfm.SessionToken,
		"track":       scrobble.Track,
		"trackNumber": strconv.Itoa(scrobble.TrackIndex),
	}

	form := url.Values{}
	for k, v := range params {
		form.Add(k, v)
	}
	form.Add("api_sig", lfm.getSig(params))
	form.Add("format", "json")

	resp, err := http.PostForm("http://ws.audioscrobbler.com/2.0/", form)

	if err != nil {
		return err
	}

	// read response to clear up socket for reuse
	bodyBytes, err := io.ReadAll(resp.Body)

	if err != nil {
		return err
	}

	logger.LogDebug(string(bodyBytes))

	return nil
}

func (lfm *lastFM) Scrobble(scrobble *Scrobble) error {
	params := map[string]string{
		"album":       scrobble.Album,
		"artist":      scrobble.Artist,
		"api_key":     ApiKey,
		"method":      "track.scrobble",
		"duration":    strconv.Itoa(scrobble.Duration),
		"sk":          lfm.SessionToken,
		"track":       scrobble.Track,
		"trackNumber": strconv.Itoa(scrobble.TrackIndex),
		"timestamp":   strconv.FormatInt(scrobble.StartTime.Unix(), 10),
	}

	form := url.Values{}
	for k, v := range params {
		form.Add(k, v)
	}
	form.Add("api_sig", lfm.getSig(params))
	form.Add("format", "json")

	resp, err := http.PostForm("http://ws.audioscrobbler.com/2.0/", form)

	if err != nil {
		return err
	}

	// read response to clear up socket for reuse
	bodyBytes, err := io.ReadAll(resp.Body)

	if err != nil {
		return err
	}

	logger.LogDebug(string(bodyBytes))

	return nil
}

func (lfm *lastFM) getSig(params map[string]string) string {
	var keys []string
	for k := range params {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	builder := strings.Builder{}

	for _, v := range keys {
		builder.WriteString(v + params[v])
	}
	builder.WriteString(SharedSecret)
	return fmt.Sprintf("%x", md5.Sum([]byte(builder.String())))
}

// FromSessionFile returns returns a lastfm session, an empty lastFMName string implies that there's only one session file in
func FromSessionFile(lastFMName string) (*lastFM, error) {
	dir, err := os.ReadDir("./")

	if err != nil {
		return nil, err
	}

	for _, file := range dir {
		if strings.HasSuffix(file.Name(), "_lastfm_session") && (strings.HasPrefix(file.Name(), lastFMName) || lastFMName == "") {
			data, err := os.ReadFile(file.Name())

			if err != nil {
				return nil, err
			}

			return &lastFM{SessionToken: string(data), Username: strings.Split(file.Name(), "_lastfm_session")[0]}, nil
		}
	}

	// user not found in sessionfile, start login prompt
	return Login()
}

func Login() (*lastFM, error) {
	logger.LogInfo("Getting auth token")
	url := "https://ws.audioscrobbler.com/2.0/?method=auth.gettoken&api_key=" + ApiKey + "&format=json"

	resp, err := http.Get(url)

	if err != nil {
		return nil, err
	}

	if resp.StatusCode == 403 {
		return nil, errors.New("login: invalid api key")
	}

	type tokenResp struct {
		Token string `json:"token"`
	}

	bodyBytes, err := io.ReadAll(resp.Body)

	if err != nil {
		return nil, err
	}

	resp.Body.Close()

	var tokenResponse tokenResp

	err = json.Unmarshal(bodyBytes, &tokenResponse)

	if err != nil {
		return nil, err
	}

	authToken := tokenResponse.Token

	logger.LogInfo("Got auth token")

	fmt.Printf("Please open https://www.last.fm/api/auth/?api_key=%s&token=%s in your browser and press enter after you're done", ApiKey, authToken)
	fmt.Scanln()

	logger.LogInfo("Fetching session")

	sigString := fmt.Sprintf("api_key%smethodauth.getSessiontoken%s%s", ApiKey, authToken, SharedSecret)
	apiSig := fmt.Sprintf("%x", md5.Sum([]byte(sigString)))

	url = fmt.Sprintf("https://ws.audioscrobbler.com/2.0/?method=auth.getSession&api_key=%s&token=%s&api_sig=%s&format=json",
		ApiKey, authToken, apiSig)

	resp, err = http.Get(url)

	if err != nil {
		return nil, err
	}

	bodyBytes, err = io.ReadAll(resp.Body)

	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 {
		logger.LogError(string(bodyBytes))
		return nil, fmt.Errorf("login: unexpected statuscode %d", resp.StatusCode)
	}

	type webSessionResp struct {
		Session struct {
			Name       string `json:"name"`
			Key        string `json:"key"`
			Subscriber int    `json:"subscriber"`
		} `json:"session"`
	}

	var webSessionResponse webSessionResp

	err = json.Unmarshal(bodyBytes, &webSessionResponse)

	if err != nil {
		return nil, err
	}

	logger.LogInfo("Got session")

	err = os.WriteFile(webSessionResponse.Session.Name+"_lastfm_session", []byte(webSessionResponse.Session.Key), 0655)

	if err != nil {
		return nil, err
	}

	return &lastFM{SessionToken: webSessionResponse.Session.Key, Username: webSessionResponse.Session.Name}, nil
}
