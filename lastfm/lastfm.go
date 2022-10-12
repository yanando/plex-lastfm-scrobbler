package lastfm

import (
	"crypto/md5"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

var (
	ApiKey       = "b8edfdf74bc415dd85755c09a0923d3f"
	SharedSecret = "2a09de6493dbf45eb11f0db2c7c97848"
)

type LastFM struct {
	SessionToken string
}

// Nowplaying, duration in seconds
func (lfm *LastFM) NowPlaying(album, artist, title string, duration, trackIndex int) error {
	params := map[string]string{
		"album":       album,
		"artist":      artist,
		"api_key":     ApiKey,
		"method":      "track.updateNowPlaying",
		"duration":    strconv.Itoa(duration),
		"sk":          lfm.SessionToken,
		"track":       title,
		"trackNumber": strconv.Itoa(trackIndex),
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
	_, err = io.ReadAll(resp.Body)

	if err != nil {
		return err
	}

	return nil
}

func (lfm *LastFM) Scrobble(album, artist, title string, duration, trackIndex int, started time.Time) error {
	params := map[string]string{
		"album": album,

		"artist":      artist,
		"api_key":     ApiKey,
		"method":      "track.scrobble",
		"duration":    strconv.Itoa(duration),
		"sk":          lfm.SessionToken,
		"track":       title,
		"trackNumber": strconv.Itoa(trackIndex),
		"timestamp":   strconv.FormatInt(started.Unix(), 10),
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

	fmt.Println(string(bodyBytes))

	return nil
}

func (lfm *LastFM) getSig(params map[string]string) string {
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
func FromSessionFile(lastFMName string) (*LastFM, error) {
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

			return &LastFM{SessionToken: string(data)}, nil
		}
	}

	// user not found in sessionfile, start login prompt
	return Login()
}

func Login() (*LastFM, error) {
	log.Println("Getting auth token")
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

	log.Println("Got auth token")

	fmt.Printf("Please open https://www.last.fm/api/auth/?api_key=%s&token=%s in your browser and press enter after you're done", ApiKey, authToken)
	fmt.Scanln()

	log.Println("Fetching session")

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
		log.Println(string(bodyBytes))
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

	log.Println("Got session")

	err = os.WriteFile(webSessionResponse.Session.Name+"_lastfm_session", []byte(webSessionResponse.Session.Key), 0655)

	if err != nil {
		return nil, err
	}

	return &LastFM{SessionToken: webSessionResponse.Session.Key}, nil
}
