package creeperkeeper

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
)

// ReadM3U returns a list of filenames from an M3U playlist.
func ReadM3U(r io.Reader) (files []string, err error) {
	s := bufio.NewScanner(r)
	for s.Scan() {
		if strings.HasPrefix(s.Text(), "#") {
			continue
		}
		filename := strings.TrimRight(s.Text(), "\r\n")
		files = append(files, filename)
	}
	return files, s.Err()
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
// their hardsub versions, if they exist.
func HardSubM3U(w io.Writer, r io.Reader) error {
	s := bufio.NewScanner(r)
	b := &bytes.Buffer{}
	for s.Scan() {
		if strings.HasPrefix(s.Text(), "#") {
			fmt.Fprintln(b, s.Text())
			continue
		}
		subbed := SubtitledVideoFilename(s.Text())
		if FileExists(subbed) {
			fmt.Fprintln(b, subbed)
		} else {
			fmt.Fprintln(b, s.Text())
		}
		_, err := fmt.Fprint(w, b.String())
		if err != nil {
			return err
		}
		b.Reset()
	}
	return nil
}

func FileExists(name string) bool {
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
	videoFiles, err := ReadM3U(f)
	if err != nil {
		return nil, err
	}
	metaFiles := make([]string, len(videoFiles))
	for i, file := range videoFiles {
		metaFiles[i] = metadataFilename(file)
	}
	return ReadAllVineMetadata(metaFiles)
}

func metadataFilename(videoFile string) string {
	return strings.TrimSuffix(videoFile, ".mp4") + ".json"
}
