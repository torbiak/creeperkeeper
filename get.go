package creeperkeeper

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

const baseURL = "https://vine.co"
const vineDateFormat = "2006-01-02T15:04:05.999999"

var fallbackUUIDCount = 0
var uuidMutex = sync.Mutex{}

var userURLRE = regexp.MustCompile(`(?:https?://)?vine\.co/(u/)?([^/]+)(/likes)?/?(\?.*)?$`)
var vineURLRE = regexp.MustCompile(`https://vine\.co/v/([a-zA-Z0-9]+)$`)

type vineExtractor func(url string) (vines []Vine, err error)

// DownloadVines downloads vines to files named after their shortIDs, eg
// bnmHnwVILKD.mp4.
func DownloadVines(vines []Vine) error {
	f := func(i interface{}) (err error) {
		vine := i.(Vine)
		file, err := os.Create(vine.VideoFilename())
		if err != nil {
			return err
		}
		defer func() {
			if cerr := file.Close(); err == nil && cerr != nil {
				err = cerr
			}
		}()
		err = vine.Download(file)
		if err != nil {
			log.Printf("get [%s] \"%.20s\": %s", vine.Uploader, vine.Title, err)
		} else if Verbose {
			log.Printf("got [%s] %s", vine.Uploader, vine.Title)
		}
		return err
	}

	// Convert []Vine to []interface{}
	jobs := make([]interface{}, len(vines))
	for i, v := range vines {
		jobs[i] = v
	}

	nerr := parallel(jobs, f, 4)
	if nerr > 0 {
		return fmt.Errorf("%d/%d failed", nerr, len(vines))
	}
	return nil
}

// ExtractVines gets vine metadata related to a url for a single vine, a user
// profile, or a user's likes. API requests are made as necessary to get all of
// a user's posts or likes.
func ExtractVines(url string) (vines []Vine, err error) {
	extractors := []vineExtractor{
		vineExtractor(vineURLToVines),
		vineExtractor(userURLToVines),
	}
	var errors []string
	for _, extractor := range extractors {
		vines, err := extractor(url)
		if err != nil {
			errors = append(errors, err.Error())
			continue
		}
		return vines, nil
	}
	return nil, fmt.Errorf("vine extraction: %s", strings.Join(errors, ", "))
}

// vineURLToVines GETs url, which is expected to the the url for a single vine,
// and extracts the vine's metadata from some JSON embedded in it.
func vineURLToVines(url string) (vines []Vine, err error) {
	matched, err := regexp.MatchString(`https?://(?:www\.)?vine\.co/(?:v|oembed)/(?P<id>\w+)`, url)
	if !matched {
		return nil, fmt.Errorf("url must be for an individual vine: %s", url)
	}
	if err != nil {
		return nil, err
	}
	// There doesn't seem to be an api endpoint for vines, so look in the html.
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return vineHTMLToVines(body)
}

func vineHTMLToVines(html []byte) (vines []Vine, err error) {
	jsonPattern := regexp.MustCompile(`window\.POST_DATA\s*=\s*({.+?});\s*</script>`)
	m := jsonPattern.FindSubmatch(html)
	if len(m) == 0 {
		return nil, fmt.Errorf("no vine metadata found in html")
	}
	var jm jsonMap
	err = json.Unmarshal(m[1], &jm)
	if err != nil {
		return nil, err
	}
	var jvine jsonVine
	for _, jvine = range jm { // Get first value from map.
		break
	}
	for _, jurl := range jvine.VideoURLs {
		if jurl.ID == "original" {
			created, err := time.Parse(vineDateFormat, jvine.Created)
			if err != nil {
				return nil, err
			}
			vines = append(vines, Vine{
				Title:      jvine.Description,
				Uploader:   jvine.Username,
				UploaderID: strconv.FormatInt(jvine.UserID, 10),
				URL:        jurl.VideoURL,
				UUID:       jvine.ShortID,
				Created:    created,
				Venue:      jvine.VenueName,
			})
			return vines, nil
		}
	}
	return nil, nil
}

func userURLToVines(url string) (vines []Vine, err error) {
	userID, err := userURLToUserID(url)
	if err != nil {
		return nil, err
	}
	m := userURLRE.FindStringSubmatch(url)
	if len(m[3]) > 0 {
		vines, err = likedVines(userID)
	} else {
		vines, err = postedVines(userID)
	}
	if err != nil {
		return nil, err
	}
	return vines, nil
}

func userURLToUserID(url string) (string, error) {
	m := userURLRE.FindStringSubmatch(url)
	if len(m) == 0 {
		return "", fmt.Errorf("unrecognized vine user url: %q", url)
	}
	isVanity := len(m[1]) == 0
	if isVanity {
		profileURL := fmt.Sprintf("%s/api/users/profiles/vanity/%s", baseURL, m[2])
		var ur userResult
		err := deserialize(profileURL, &ur)
		if err != nil {
			return "", err
		}
		return fmt.Sprint(ur.Data.UserID), nil
	} else {
		return m[2], nil
	}
}

func likedVines(userID string) (vines []Vine, err error) {
	url := fmt.Sprintf("%s/api/timelines/users/%s/likes", baseURL, userID)
	return timelineVines(url)
}

func postedVines(userID string) (vines []Vine, err error) {
	url := fmt.Sprintf("%s/api/timelines/users/%s", baseURL, userID)
	return timelineVines(url)
}

func timelineVines(url string) (vines []Vine, err error) {
	more := true
	for i := 1; more; i++ {
		urlWithParams := fmt.Sprintf("%s?page=%d&size=100", url, i)
		var pageVines []Vine
		pageVines, more, err = timelinePageVines(urlWithParams)
		if err != nil {
			return nil, err
		}
		vines = append(vines, pageVines...)
	}
	return vines, nil
}

func timelinePageVines(url string) (vines []Vine, more bool, err error) {
	var tr timelineResult
	err = deserialize(url, &tr)
	if err != nil {
		return nil, false, err
	}
	for _, tv := range tr.Data.Records {
		created, err := time.Parse(vineDateFormat, tv.Created)
		if err != nil {
			return nil, false, err
		}
		vines = append(vines, Vine{
			Title:      tv.Description,
			Uploader:   tv.Username,
			UploaderID: strconv.FormatInt(tv.UserID, 10),
			URL:        tv.VideoURL,
			UUID:       vineURLToUUID(tv.PermalinkURL),
			Created:    created,
			Venue:      tv.VenueName,
		})
	}
	more = tr.Data.NextPage > 0
	return vines, more, nil
}

// deserialize GETs a JSON API endpoint, unwraps the enveloping object and
// unmarshals the response.
func deserialize(url string, d interface{}) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	var env jsonVineEnvelope
	err = json.Unmarshal(body, &env)
	if err != nil {
		return fmt.Errorf("unrecognized json: %s", body)
	}
	if !env.Success {
		return fmt.Errorf("GET %q: status %d: %s", url, resp.StatusCode, env.Error)
	}
	err = json.Unmarshal(body, &d)
	if err != nil {
		return err
	}
	return nil
}

func vineURLToUUID(url string) string {
	m := vineURLRE.FindStringSubmatch(url)
	if len(m) == 0 {
		uuidMutex.Lock()
		defer uuidMutex.Unlock()
		return fmt.Sprintf("fallbackID%d", fallbackUUIDCount)
	}
	return string(m[1])
}

func FilterOutReposts(vines []Vine, url string) ([]Vine, error) {
	userid, err := userURLToUserID(url)
	if err != nil {
		return vines, err
	}
	filtered := []Vine{}
	for _, vine := range vines {
		if vine.UploaderID != userid {
			continue
		}
		filtered = append(filtered, vine)
	}
	return filtered, nil
}

type jsonVineEnvelope struct {
	Success bool
	Error   string
}

// Single-Vine JSON structures
type jsonMap map[string]jsonVine
type jsonVine struct {
	Username    string
	UserID      int64
	Description string
	ShortID     string
	VideoURLs   []jsonVideoURL
	VenueName   string
	Created     string
}
type jsonVideoURL struct {
	VideoURL string
	ID       string
}

// Timeline (posts/likes) API JSON structures
type timelineResult struct {
	Data    timelineRecords
	Success bool
	Error   string
}
type timelineRecords struct {
	Records  []timelineVine
	NextPage int
}
type timelineVine struct {
	Username     string
	Description  string
	VideoURL     string
	PermalinkURL string
	VenueName    string
	Created      string
	UserID       int64
}

// User API JSON structures
type userResult struct {
	Data jsonUser
}
type jsonUser struct {
	UserID int64
}
