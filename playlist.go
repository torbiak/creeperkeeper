package creeperkeeper

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"regexp"
	"strings"
)

var noSubsRE = regexp.MustCompile("^# *nosubtitles")

type plItem struct {
	Filename    string
	NoSubtitles bool
}

func ReadM3U(r io.Reader) (playlist []plItem, err error) {
	s := bufio.NewScanner(r)
	noSub := false
	for s.Scan() {
		if strings.HasPrefix(s.Text(), "#") {
			if !noSub && noSubsRE.MatchString(s.Text()) {
				noSub = true
			}
			continue
		}
		filename := strings.TrimRight(s.Text(), "\r\n")
		playlist = append(playlist, plItem{filename, noSub})
		noSub = false
	}
	return playlist, s.Err()
}

func WriteM3U(w io.Writer, vines []Vine) error {
	_, err := fmt.Fprintln(w, "#EXTM3U")
	if err != nil {
		return err
	}
	for _, vine := range vines {
		_, err = fmt.Fprintln(w, vine.M3UEntry())
		if err != nil {
			return err
		}
	}
	return nil
}

// HardSubM3U takes an existing M3U playlist as r and replaces videos with
// their hardsub versions, if they exist and the entry isn't preceded by a
// nosubtitles annotation.
func HardSubM3U(w io.Writer, r io.Reader) error {
	s := bufio.NewScanner(r)
	noSubs := false
	b := &bytes.Buffer{}
	for s.Scan() {
		if noSubsRE.MatchString(s.Text()) {
			noSubs = true
		}
		if strings.HasPrefix(s.Text(), "#") {
			fmt.Fprintln(b, s.Text())
			continue
		}
		subbed := subtitledVideoFilename(s.Text())
		if noSubs || !fileExists(subbed) {
			fmt.Fprintln(b, s.Text())
		} else {
			fmt.Fprintln(b, subbed)
		}
		_, err := fmt.Fprint(w, b.String())
		if err != nil {
			return err
		}
		noSubs = false
		b.Reset()
	}
	return nil
}

func fileExists(name string) bool {
	_, err := os.Stat(name)
	if os.IsNotExist(err) {
		return false
	}
	if err != nil {
		log.Println(err)
	}
	return true
}

func ReadMetadataForPlaylist(playlist string) ([]Vine, error) {
	f, err := os.Open(playlist)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	plItems, err := ReadM3U(f)
	if err != nil {
		return nil, err
	}
	videoFiles := make([]string, len(plItems))
	for i, item := range plItems {
		videoFiles[i] = metadataFilename(item.Filename)
	}
	return ReadAllVineMetadata(videoFiles)
}

func metadataFilename(videoFile string) string {
	return strings.TrimSuffix(videoFile, ".mp4") + ".json"
}
