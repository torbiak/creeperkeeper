package creeperkeeper

import (
	"log"
	"os"
	"os/exec"
	"fmt"
	"strconv"
	"regexp"
	"path/filepath"
	"io"
	"io/ioutil"
	"runtime"
)

var widthRE = regexp.MustCompile(`streams\.stream\.\d+\.width=(\d+)`)
var heightRE = regexp.MustCompile(`streams\.stream\.\d+\.height=(\d+)`)

// ScaleAll normalizes all videos to 720x720.
func ScaleAll(files []string) error {
	jobs, err := NeedScaling(files)
	if err != nil {
		return err
	}
	f := func(i interface{}) error {
		file := i.(string)
		err :=  scale(file)
		if err != nil {
			log.Printf("scale %s: %s", file, err)
		}
		return err
	}
	interfaces := make([]interface{}, len(jobs))
	for i, job := range jobs {
		interfaces[i] = job
	}
	nerr := parallel(interfaces, f, runtime.NumCPU())
	if nerr != 0 {
		return fmt.Errorf("%d/%d failed", nerr, len(jobs))
	}
	return nil
}

func NeedScaling(files []string) ([]string, error) {
	need := []string{}
	nerr := 0
	for _, file := range files {
		w, h, err := videoDimensions(file)
		if err != nil {
			nerr++
			log.Printf("get dimensions for %s: %s", file, err)
			continue
		}
		if w != 720 || h != 720 {
			need = append(need, file)
		}
	}
	var err error
	if nerr > 0 {
		err = fmt.Errorf("get dimensions: %d/%d failed", nerr, len(files))
	}
	return need, err
}

func videoDimensions(file string) (width int, height int, err error) {
	cmd := exec.Command(
		"ffprobe",
		"-v", "warning", 
		"-show_streams",
		"-of", "flat",
		file)
	stdout, err := runCmd(cmd)
	if err != nil {
		return
	}

	m := widthRE.FindSubmatch(stdout)
	if m == nil {
		err = fmt.Errorf("no width in ffprobe output: %q", stdout)
		return 
	}
	width, err = strconv.Atoi(string(m[1]))
	if err != nil {
		return
	}

	m = heightRE.FindSubmatch(stdout)
	if m == nil {
		err = fmt.Errorf("no height in ffprobe output: %q", stdout)
		return 
	}
	height, err = strconv.Atoi(string(m[1]))
	return
}

// Scale a video to standard dimensions (720x720). Some vines are only
// available at 480x480.
func scale(file string) error {
	dir, err := ioutil.TempDir("", "crkr_scale")
	if err != nil {
		return err
	}
	defer func() {
		if err := os.RemoveAll(dir); err != nil {
			log.Printf("remove scale temp dir: %s", err)
		}
	}()
	scaled := filepath.Join(dir, "scaled.mp4")
	cmd := exec.Command(
		"ffmpeg",
		"-v", "warning",
		"-i", file,
		"-vf", "scale=720:720",
		scaled)
	_, err = runCmd(cmd)
	if err != nil {
		return err
	}
	return moveFile(scaled, file)
}

func moveFile(dst, src string) (err error) {
	// Try a cheap rename first.
	err = os.Rename(dst, src)
	if err == nil {
		return
	}
	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer func() {
		if cerr := dstFile.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()
	_, err = io.Copy(dstFile, srcFile)
	if err != nil {
		return err
	}
	return os.Remove(src)
}
