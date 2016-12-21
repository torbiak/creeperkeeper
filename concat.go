package creeperkeeper

import (
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ConcatVideos joins mp4 videos losslessly, like so:
// ffmpeg -i input1.mp4 -c copy -bsf:v h264_mp4toannexb -f mpegts intermediate1.ts
// ffmpeg -i input2.mp4 -c copy -bsf:v h264_mp4toannexb -f mpegts intermediate2.ts
// ffmpeg -i "concat:intermediate1.ts|intermediate2.ts" -c copy -bsf:a aac_adtstoasc output.mp4
func ConcatVideos(videoFiles []string, outFile string) error {
	dir, err := ioutil.TempDir("", "crkr")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)

	tsFiles := []string{}
	for _, f := range videoFiles {
		base := filepath.Base(f)
		tsFile := filepath.Join(dir, strings.TrimSuffix(base, ".mp4")+".ts")
		tsFiles = append(tsFiles, tsFile)
		err := mp4ToTransportStream(f, tsFile)
		if err != nil {
			return err
		}
	}
	cmd := exec.Command(
		"ffmpeg",
		"-y",
		"-v", "warning",
		"-i", "concat:"+strings.Join(tsFiles, "|"),
		"-c", "copy",
		"-bsf:a", "aac_adtstoasc",
		outFile)
	return runCmd(cmd)
}

func mp4ToTransportStream(inFile, outFile string) error {
	cmd := exec.Command(
		"ffmpeg",
		"-y", // Support multiple instances of the same video in a playlist.
		"-v", "warning",
		"-i", inFile,
		"-c", "copy",
		"-bsf:v", "h264_mp4toannexb",
		"-shortest",
		"-f", "mpegts",
		outFile)
	return runCmd(cmd)
}
