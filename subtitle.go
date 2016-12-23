package creeperkeeper

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"os/exec"
	"runtime"
	"strings"
	"text/template"
	"time"
	"unicode"
	"unicode/utf8"
)

func WriteSubtitles(vines []Vine, t time.Duration, tmpl *template.Template, plainEmoji bool) error {
	var nerrors = 0
	for _, vine := range vines {
		subs, err := vine.Subtitles(t, tmpl)
		if err != nil {
			nerrors += 1
			log.Printf("write subtitles for %s: %s", vine.UUID, err)
			// Maybe we should return on a template.ExecError?
			continue
		}
		if plainEmoji {
			subs = removeEmojiVariationSelectors(subs)
		}
		err = ioutil.WriteFile(vine.SubtitlesFilename(), []byte(subs), 0666)
		if err != nil {
			nerrors += 1
			log.Printf("write subtitles for %s: %s", vine.UUID, err)
		}
	}
	if nerrors > 0 {
		return fmt.Errorf("write subtitles: %d/%d failed", nerrors, len(vines))
	}
	return nil
}

func RenderAllSubtitles(filenames []string, fontName string, fontSize int) error {
	// A proactive check to avoid getting an error for every video.
	_, err := exec.LookPath("ffmpeg")
	if err != nil {
		return fmt.Errorf("ffmpeg not found in PATH")
	}

	f := func(i interface{}) error {
		video := i.(string)
		subbed := SubtitledVideoFilename(video)
		return RenderSubtitles(subbed, video, fontName, fontSize)
	}

	// Convert []sting to []interface{}
	jobs := make([]interface{}, len(filenames))
	for i, f := range filenames {
		jobs[i] = f
	}

	nerr := parallel(jobs, f, runtime.NumCPU())
	if nerr > 0 {
		return fmt.Errorf("render subtitles: %d/%d failed", nerr, len(filenames))
	}
	return nil
}

// RenderSubtitles overlays subtitles in ${videoFile%.mp4}.srt on file to
// produce ${videoFile%.mp4}.sub.mp4
func RenderSubtitles(outFile, videoFile, fontName string, fontSize int) error {
	basename := strings.TrimSuffix(videoFile, ".mp4")
	subtitles := basename + ".srt"
	subtitledVideo := SubtitledVideoFilename(videoFile)
	style := fmt.Sprintf(
		"subtitles=f=%s:force_style='FontName=%s,Fontsize=%d'",
		subtitles, fontName, fontSize)
	cmd := exec.Command(
		"ffmpeg",
		"-y",
		"-v", "warning",
		"-i", videoFile,
		"-vf", style,
		subtitledVideo)
	_, err := runCmd(cmd)
	return err
}

func removeEmojiVariationSelectors(s string) string {
	b := &bytes.Buffer{}
	for len(s) > 0 {
		r, size := utf8.DecodeRuneInString(s)
		s = s[size:]
		if unicode.Is(unicode.Properties["Variation_Selector"], r) {
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

func SubtitledVideoFilename(videoFile string) string {
	return strings.TrimSuffix(videoFile, ".mp4") + ".sub.mp4"
}
