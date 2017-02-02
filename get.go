package creeperkeeper

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"
)

const vineDateFormat = "2006-01-02T15:04:05.999999"

var fallbackUUIDCount = 0
var uuidMutex = sync.Mutex{}

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
	extractors := map[string]vineExtractor{
		"individual": vineExtractor(vineURLToVines),
		"user":       vineExtractor(userURLToVines),
	}
	var errors []string
	for name, extractor := range extractors {
		vines, err := extractor(url)
		if err != nil {
			errors = append(errors, fmt.Sprintf("%s: %s", name, err.Error()))
			continue
		}
		return vines, nil
	}
	return nil, fmt.Errorf("vine extraction: %s", strings.Join(errors, "\n"))
}

// vineURLToVines gets vine metadata for the vine referred to by the given URL.
func vineURLToVines(url string) (vines []Vine, err error) {
	vineURLRE := regexp.MustCompile(`https?://(?:www\.)?vine\.co/(?:v|oembed)/([^?/]+)`)
	m := vineURLRE.FindStringSubmatch(url)
	if len(m) == 0 {
		return nil, fmt.Errorf("vineURLToVines: unrecognized url: %s", url)
	}
	id := m[1]
	vine, err := getVine(id)
	if err != nil {
		return nil, err
	}
	return []Vine{vine}, nil
}

func getVine(id string) (Vine, error) {
	var jv jsonVine
	url := fmt.Sprintf("https://archive.vine.co/posts/%s.json", id)
	err := deserialize(url, &jv)
	if err != nil {
		return Vine{}, fmt.Errorf("getVine: %s", err)
	}
	created, err := time.Parse(vineDateFormat, jv.Created)
	if err != nil {
		return Vine{}, fmt.Errorf("getVine: %s", err)
	}
	return Vine{
		Title:      jv.Description,
		Uploader:   jv.Username,
		UploaderID: jv.UserIdStr,
		URL:        jv.VideoURL,
		UUID:       id,
		Created:    created,
	}, nil
}

func userURLToVines(url string) ([]Vine, error) {
	userID, err := userURLToUserID(url)
	if err != nil {
		return nil, fmt.Errorf("userURLToVines: %s", err)
	}
	var ju jsonUser
	postsURL := fmt.Sprintf("https://archive.vine.co/profiles/%s.json", userID)
	err = deserialize(postsURL, &ju)
	if err != nil {
		return nil, fmt.Errorf("userURLToVines: %s", err)
	}

	var vines []Vine
	vineq := make(chan Vine)
	wg := sync.WaitGroup{}
	go func() {
		wg.Add(1)
		for vine := range vineq {
			vines = append(vines, vine)
		}
		wg.Done()
	}()
	jobs := make([]interface{}, len(ju.Posts))
	for i, v := range ju.Posts {
		jobs[i] = v
	}
	f := func(i interface{}) error {
		id := i.(string)
		vine, err := getVine(id)
		if err != nil {
			log.Printf("get vine metadata for %s: %s", id, err)
			return err
		}
		vineq <- vine
		return nil
	}
	nerr := parallel(jobs, f, 8)
	close(vineq)
	wg.Wait()
	if nerr > 0 {
		return vines, fmt.Errorf("get vine metadata: %d/%d failed", nerr, len(ju.Posts))
	}
	return vines, nil
}

func userURLToUserID(url string) (string, error) {
	userURLRE := regexp.MustCompile(`(?:https?://)?vine\.co/(u/)?([^/]+)/?(?:\?.*)?$`)
	m := userURLRE.FindStringSubmatch(url)
	if len(m) == 0 {
		return "", fmt.Errorf("unrecognized vine user url: %q", url)
	}
	isVanity := len(m[1]) == 0
	if isVanity {
		profileURL := fmt.Sprintf("https://vine.co/api/users/profiles/vanity/%s", m[2])
		var jve jsonVanityEnvelope
		err := deserialize(profileURL, &jve)
		if err != nil {
			return "", err
		}
		return fmt.Sprint(jve.Data.UserID), nil
	} else {
		return m[2], nil
	}
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
	if resp.StatusCode != 200 {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, body)
	}
	err = json.Unmarshal(body, &d)
	if err != nil {
		return fmt.Errorf("unrecognized json %s", err)
	}
	return nil
}

func vineURLToUUID(url string) string {
	vineURLRE := regexp.MustCompile(`https://vine\.co/v/([a-zA-Z0-9]+)$`)
	m := vineURLRE.FindStringSubmatch(url)
	if len(m) == 0 {
		uuidMutex.Lock()
		defer uuidMutex.Unlock()
		return fmt.Sprintf("fallbackID%d", fallbackUUIDCount)
	}
	return string(m[1])
}

type jsonUser struct {
	Posts []string
}

type jsonVine struct {
	Description string
	Username    string
	UserIdStr   string
	VideoURL    string
	Created     string
}

// User API JSON structures
type jsonVanityEnvelope struct {
	Data jsonVanity
}
type jsonVanity struct {
	UserID int64
}
