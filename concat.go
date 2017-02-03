package creeperkeeper

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ConcatVideos joins mp4 videos losslessly, like so:
// ffmpeg -i input1.mp4 -c copy -bsf:v h264_mp4toannexb -f mpegts intermediate1.ts
// ffmpeg -i input2.mp4 -c copy -bsf:v h264_mp4toannexb -f mpegts intermediate2.ts
// for f in *.ts; do printf "file '%s'\n" $f; done >files
// ffmpeg -f concat -i files -c copy output.mp4
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

	tmpFile, err := ioutil.TempFile(dir, "crkr_concat")
	if err != nil {
		return err
	}
	defer os.Remove(tmpFile.Name())
	for _, f := range tsFiles {
		_, err := fmt.Fprintf(tmpFile, "file '%s'\n", f)
		if err != nil {
			tmpFile.Close()
			return err
		}
	}
	err = tmpFile.Close()
	if err != nil {
		return err
	}

	cmd := exec.Command(
		"ffmpeg",
		"-y",
		"-v", "warning",
		"-f", "concat",
		// "safe" filenames are relative and can only consist of
		// [a-zA-Z0-9_.-].
		"-safe", "0",
		"-i", tmpFile.Name(),
		"-c", "copy",
		outFile)
	_, err = runCmd(cmd)
	return err
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
	_, err := runCmd(cmd)
	return err
}
