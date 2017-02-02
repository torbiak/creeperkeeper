package creeperkeeper

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"text/template"
	"time"
	"unicode"
	"unicode/utf8"
)

const fontconfig = `<?xml version="1.0"?>
<!DOCTYPE fontconfig SYSTEM "fonts.dtd">
<fontconfig>
    <dir>C:\Windows\Fonts</dir>
    <dir>C:\WINNT\Fonts</dir>
    <cachedir>~/.fontconfig</cachedir>
</fontconfig>`

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

	if runtime.GOOS == "windows" {
		filenames, err = relativePaths(filenames)
		if err != nil {
			return fmt.Errorf("give path to subtitles filter: %s", err)
		}
	}

	f := func(i interface{}) error {
		video := i.(string)
		subbed := SubtitledVideoFilename(video)
		return RenderSubtitles(subbed, video, fontName, fontSize)
	}

	// Convert []string to []interface{}
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
	err := configureFontConfig(cmd)
	if err != nil {
		return err
	}
	_, err = runCmd(cmd)
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

func relativePaths(filenames []string) ([]string, error) {
	relpaths := make([]string, len(filenames))
	wd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	for i, f := range filenames {
		abs, err := filepath.Abs(f)
		if err != nil {
			return nil, err
		}
		rel, err := filepath.Rel(wd, abs)
		if err != nil {
			return nil, err
		}
		relpaths[i] = filepath.ToSlash(rel)
	}
	return relpaths, nil
}

// On unix systems fontconfig will usually be installed by a package manager
// and a config file will have been written to a well-known path. But on
// windows fontconfig is most likely provided by ffmpeg and will not be
// configured. Some versions of ffmpeg that I've used have mostly reasonable
// fallback settings and the subtitles filter works as long as
// force_style='Fontname=<font>' is specified, but others don't, so in the
// interests of saving users frustration, write a basic fontconfig config file
// to the user's homedir and set FONTCONFIG_FILE to point at it.
func configureFontConfig(cmd *exec.Cmd) error {
	// Return early if fontconfig is probably already configured.
	if runtime.GOOS != "windows" {
		return nil
	}
	if os.Getenv("FONTCONFIG_FILE") != "" {
		return nil
	}

	// Determine the file location.
	homedir, err := windowsHome()
	if err != nil {
		return fmt.Errorf("find home dir: %s", err)
	}
	configFile := filepath.Join(homedir, ".fonts.conf")

	// Change the environment.
	env := os.Environ()
	env = append(env, "FONTCONFIG_FILE="+configFile)
	cmd.Env = env

	// Write the file if needed.
	if _, err := os.Stat(configFile); err == nil {
		return nil
	}
	return ioutil.WriteFile(configFile, []byte(fontconfig), 0644)
}

func windowsHome() (string, error) {
	if home := os.Getenv("HOME"); home != "" {
		return home, nil
	}
	drive, path := os.Getenv("HOMEDRIVE"), os.Getenv("HOMEPATH")
	if drive != "" && path != "" {
		return drive + path, nil
	}
	if profile := os.Getenv("USERPROFILE"); profile != "" {
		return profile, nil
	}
	return "", fmt.Errorf("HOME, HOMEDRIVE, HOMEPATH, and USERPROFILE are empty")
}
