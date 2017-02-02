package creeperkeeper

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"text/template"
	"time"
)

var Verbose = false

type Vine struct {
	Title      string
	Uploader   string
	UploaderID string
	URL        string
	UUID       string
	Created    time.Time
}

func (v Vine) Download(w io.Writer) error {
	resp, err := http.Get(v.URL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, err = io.Copy(w, resp.Body)
	if err != nil {
		return fmt.Errorf("download %s: %s", v.URL, err)
	}
	return nil
}

func (v Vine) VideoFilename() string {
	return v.UUID + ".mp4"
}

func (v Vine) SubtitlesFilename() string {
	return v.UUID + ".srt"
}

func (v Vine) MetadataFilename() string {
	return v.UUID + ".json"
}

func (v Vine) Subtitles(t time.Duration, tmpl *template.Template) (string, error) {
	stopTime := fmt.Sprintf("%02.0f:%02.0f:%02.0f,%03d", t.Hours(), t.Minutes(), t.Seconds(), (t.Nanoseconds()/1e6)%1000)

	b := &bytes.Buffer{}
	err := tmpl.Execute(b, v)
	if err != nil {
		return "", err
	}

	// Remove blank lines that would otherwise prematurely signal the end of
	// the subtitle.
	subtitle := strings.NewReplacer("\r", "", "\n\n", "\n").Replace(b.String())

	subtitle = strings.TrimSpace(subtitle)
	return fmt.Sprintf("1\n00:00:00,000 --> %s\n%s\n", stopTime, subtitle), nil
}

// M3UEntry returns an extended M3U entry.
func (v Vine) M3UEntry() string {
	// Cram the title onto one line.
	title := v.Title
	title = strings.Replace(title, "\r", "", -1)
	title = strings.Replace(title, "\n", " ", -1)
	return fmt.Sprintf("#EXTINF:-1,%s: %s\n%s", v.Uploader, title, v.VideoFilename())
}

func ReadAllVineMetadata(filenames []string) ([]Vine, error) {
	nerr := 0
	vines := make([]Vine, len(filenames))
	for i, filename := range filenames {
		var err error
		vines[i], err = ReadVineMetadata(filename)
		if err != nil {
			nerr++
			log.Printf("read metadata from %s: %s", filename, err)
		}
	}
	if nerr > 0 {
		return vines, fmt.Errorf("%d/%d failed", nerr, len(filenames))
	}
	return vines, nil
}

func ReadVineMetadata(filename string) (Vine, error) {
	var vine Vine
	f, err := os.Open(filename)
	if err != nil {
		return vine, err
	}
	defer f.Close()
	dec := json.NewDecoder(f)
	err = dec.Decode(&vine)
	if err != nil {
		return vine, err
	}
	return vine, err
}

func WriteAllVineMetadata(vines []Vine) error {
	nerr := 0
	for _, vine := range vines {
		err := WriteVineMetadata(vine)
		if err != nil {
			log.Printf("write metadata for %s: %s", vine.UUID, err)
			nerr++
		}
	}
	if nerr > 0 {
		return fmt.Errorf("%d/%d failed", nerr, len(vines))
	}
	return nil
}

func WriteVineMetadata(vine Vine) error {
	f, err := os.Create(vine.MetadataFilename())
	if err != nil {
		return err
	}
	defer func() {
		if cerr := f.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()
	enc := json.NewEncoder(f)
	return enc.Encode(vine)
}
