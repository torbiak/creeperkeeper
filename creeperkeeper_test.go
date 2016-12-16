package creeperkeeper

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"text/template"
	"time"
)

var subTemplate *template.Template

func init() {
	var err error
	subTemplate, err = template.New("subtitles").Parse("{{.Title}}")
	if err != nil {
		panic(err)
	}
}

func TestExtractVines_singleVine(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping long test")
	}
	got, err := ExtractVines("https://vine.co/v/b9KOOWX7HUx")
	if err != nil {
		t.Error(err)
	}
	want := []Vine{
		{
			Title:    "Chicken.",
			Uploader: "Jack",
			URL:      "http://v.cdn.vine.co/v/videos/7508FF74-000E-48F3-9E5E-D7218D5F8FFB-7580-000002B248D861FF_1.1.mp4?versionId=6LYxpZTqmX86aZ9uq5b8e.PqOWi45U9T",
			UUID:     "b9KOOWX7HUx",
			Venue:    "The Fremont Diner",
			Created:  time.Date(2013, 5, 19, 21, 12, 31, 0, time.UTC),
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestExtractVines_userPosts(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping long test")
	}
	vines, err := ExtractVines("https://vine.co/Mielmonster")
	if err != nil {
		t.Fatal(err)
	}
	min := 250
	if len(vines) < min {
		t.Errorf("got %d, want %d or more", len(vines), min)
	}
	last := vines[len(vines)-1]
	want := Vine{
		Title:    "Guys be like #superbowl #sexism #relatable",
		Uploader: "ig/yt: mielmonster",
		URL:      "http://mtc.cdn.vine.co/r/videos/B3748976821041887404950200320_11f822db164.4.7.7604697388532430910_B7HPE9tNTc1NVJWW.uogDgFWflxqpHF4exxF3SWcrtpPkMtCSDp6vSp.QWqMgrI_.mp4?versionId=3WCYDYMNNfVZnRTXBWB.CHYN3JhMYdoJ",
		UUID:     "Mz2Wzi73VnI",
		Venue:    "The Bro Palace - Los Angeles Branch",
		Created:  time.Date(2014, 2, 2, 22, 22, 34, 0, time.UTC),
	}
	if !reflect.DeepEqual(last, want) {
		t.Errorf("got %v, want %v", last, want)
	}
}

func TestExtractVines_userLikes(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping long test")
	}
	vines, err := ExtractVines("https://vine.co/u/1108673385032228864/likes")
	if err != nil {
		t.Fatal(err)
	}
	min := 400
	if len(vines) < min {
		t.Errorf("got %d, want %d or more", len(vines), min)
	}
	last := vines[len(vines)-1]
	want := Vine{
		Title:    "Idiots Assemble!",
		Uploader: "Ben Willbond",
		URL:      "http://mtc.cdn.vine.co/r/videos/97BE877A-800C-4991-94A5-37CACA2DB44C-137-0000000D404033B5_1.0.4.mp4?versionId=QAVqOEAfFf_LXlwpjjXmlvai7qOmxhoj",
		UUID:     "bnmHnwVILKD",
		Created:  time.Date(2013, 2, 5, 11, 6, 16, 0, time.UTC),
	}
	if !reflect.DeepEqual(last, want) {
		t.Errorf("got %v, want %v", last, want)
	}
}

func TestExtractVines_apiError(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping long test")
	}
	_, err := ExtractVines("https://vine.co/api/users/profiles/vanity/torbiak")
	if err == nil {
		t.Error("error expected for nonexistant vanity url")
	}
}

func TestWriteM3U(t *testing.T) {
	vines := []Vine{
		{
			Title:    "Guys be like #superbowl #sexism #relatable",
			Uploader: "ig/yt: mielmonster",
			URL:      "http://mtc.cdn.vine.co/r/videos/B3748976821041887404950200320_11f822db164.4.7.7604697388532430910_B7HPE9tNTc1NVJWW.uogDgFWflxqpHF4exxF3SWcrtpPkMtCSDp6vSp.QWqMgrI_.mp4?versionId=3WCYDYMNNfVZnRTXBWB.CHYN3JhMYdoJ",
			UUID:     "Mz2Wzi73VnI",
		},
		{
			Title:    "Idiots Assemble!",
			Uploader: "Ben Willbond",
			URL:      "http://mtc.cdn.vine.co/r/videos/97BE877A-800C-4991-94A5-37CACA2DB44C-137-0000000D404033B5_1.0.4.mp4?versionId=QAVqOEAfFf_LXlwpjjXmlvai7qOmxhoj",
			UUID:     "bnmHnwVILKD",
		},
	}
	buf := &bytes.Buffer{}
	err := WriteM3U(buf, vines)
	if err != nil {
		t.Fatal(err)
	}
	want := `#EXTM3U
#EXTINF:-1,ig/yt: mielmonster: Guys be like #superbowl #sexism #relatable
Mz2Wzi73VnI.mp4
#EXTINF:-1,Ben Willbond: Idiots Assemble!
bnmHnwVILKD.mp4
`
	got := buf.String()
	if buf.String() != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestWriteSubtitles(t *testing.T) {
	vines := []Vine{
		{
			Title:    "Guys be like #superbowl #sexism #relatable",
			Uploader: "ig/yt: mielmonster",
			URL:      "http://mtc.cdn.vine.co/r/videos/B3748976821041887404950200320_11f822db164.4.7.7604697388532430910_B7HPE9tNTc1NVJWW.uogDgFWflxqpHF4exxF3SWcrtpPkMtCSDp6vSp.QWqMgrI_.mp4?versionId=3WCYDYMNNfVZnRTXBWB.CHYN3JhMYdoJ",
			UUID:     "Mz2Wzi73VnI",
		},
		{
			Title:    "Idiots Assemble!",
			Uploader: "Ben Willbond",
			URL:      "http://mtc.cdn.vine.co/r/videos/97BE877A-800C-4991-94A5-37CACA2DB44C-137-0000000D404033B5_1.0.4.mp4?versionId=QAVqOEAfFf_LXlwpjjXmlvai7qOmxhoj",
			UUID:     "bnmHnwVILKD",
		},
	}
	err := WriteSubtitles(vines, 1*time.Second, subTemplate)
	if err != nil {
		t.Fatal(err)
	}
	for _, vine := range vines {
		err := os.Remove(vine.SubtitlesFilename())
		if err != nil {
			t.Error(err)
		}
	}
}

func TestVineSubtitles(t *testing.T) {
	vine := Vine{
		Title:    "Idiots Assemble!",
		Uploader: "Ben Willbond",
		URL:      "http://mtc.cdn.vine.co/r/videos/97BE877A-800C-4991-94A5-37CACA2DB44C-137-0000000D404033B5_1.0.4.mp4?versionId=QAVqOEAfFf_LXlwpjjXmlvai7qOmxhoj",
		UUID:     "bnmHnwVILKD",
	}
	got, err := vine.Subtitles(2*time.Second, subTemplate)
	if err != nil {
		t.Fatal(err)
	}
	want := "1\n00:00:00,000 --> 00:00:02,000\nIdiots Assemble!"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestReadM3U(t *testing.T) {
	playlist := `#EXTM3U
#EXTINF:-1,Guys be like #superbowl #sexism #relatable
Mz2Wzi73VnI.mp4
#nosubtitles
#EXTINF:-1,Idiots Assemble!
bnmHnwVILKD.mp4`
	want := []plItem{
		{"Mz2Wzi73VnI.mp4", false},
		{"bnmHnwVILKD.mp4", true},
	}
	got, err := ReadM3U(strings.NewReader(playlist))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(want, got) {
		t.Errorf("want %v, got %v", want, got)
	}
}

func TestRenderAllSubtitles(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping long test")
	}
	dir, err := ioutil.TempDir("", "crkr_renderAll")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	basename := "blank"
	videoFile := filepath.Join(dir, basename+".mp4")
	subbedFile := filepath.Join(dir, basename+".sub.mp4")
	subFile := filepath.Join(dir, basename+".srt")

	writeBlankVideo(t, videoFile)

	// Write subtitles to a file.
	subtitles := "1\n00:00:00,000 --> 00:00:02,000\nIdiots Assemble!"
	f, err := os.Create(subFile)
	if err != nil {
		t.Fatal(err)
	}
	_, err = io.WriteString(f, subtitles)
	if err != nil {
		t.Fatal(err)
	}

	err = RenderAllSubtitles([]string{videoFile}, 8)
	if err != nil {
		t.Fatal(err)
	}

	// Check the video has been changed, at least.
	origHash, err := videoHash(videoFile)
	if err != nil {
		t.Fatal(err)
	}
	subbedHash, err := videoHash(subbedFile)
	if err != nil {
		t.Fatal(err)
	}
	if origHash == subbedHash {
		t.Fatal("subtitled file has same hash as the input video file")
	}
}

func writeBlankVideo(t *testing.T, filename string) {
	cmd := exec.Command(
		"ffmpeg",
		"-y",
		"-loglevel", "warning",
		"-t", "6",
		"-f", "lavfi",
		"-i", "color=BLACK:720x720",
		filename)
	_, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
}

func videoHash(filepath string) (string, error) {
	buf := &bytes.Buffer{}
	cmd := exec.Command("ffmpeg",
		"-loglevel", "warning",
		"-i", filepath,
		"-f", "hash",
		"-hash", "md5",
		"-")
	cmd.Stdout = buf
	err := cmd.Run()
	return buf.String(), err
}

func TestConcatVideos(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping long test")
	}
	files := []string{"blank1.mp4", "blank2.mp4"}
	defer remove(t, files...)
	for i := 1; i <= 2; i++ {
		writeBlankVideo(t, fmt.Sprintf("blank%d.mp4", i))
	}
	outputVideo := "blank3.mp4"
	err := ConcatVideos(files, outputVideo)
	defer remove(t, outputVideo)
	if err != nil {
		t.Fatal(err)
	}
	if filesize(t, files[0]) >= filesize(t, outputVideo) {
		t.Errorf("concatenated video smaller than expected")
	}
}

func filesize(t *testing.T, filename string) int64 {
	info, err := os.Stat(filename)
	if err != nil {
		t.Fatal(err)
	}
	return info.Size()
}

func remove(t *testing.T, files ...string) {
	for _, f := range files {
		err := os.RemoveAll(f)
		if err != nil {
			t.Error(err)
		}
	}
}

func TestConcatSubtitles(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping long test")
	}
	subTemplate := "1\n00:00:00,000 --> 00:00:02,000\n%s line1\nline2\n"
	dir, err := ioutil.TempDir("", "crkr_concat")
	if err != nil {
		t.Fatal(err)
	}
	defer remove(t, dir)
	videos := []plItem{}
	for i := 0; i < 4; i++ {
		video := filepath.Join(dir, fmt.Sprintf("v%d.mp4", i))
		writeBlankVideo(t, video)
		videos = append(videos, plItem{video, false})
	}
	writeFile(t, filepath.Join(dir, "v1.srt"), fmt.Sprintf(subTemplate, "v1"))
	writeFile(t, filepath.Join(dir, "v3.srt"), fmt.Sprintf(subTemplate, "v3"))
	b := &bytes.Buffer{}
	err = ConcatSubtitles(b, videos)
	if err != nil {
		t.Fatal(err)
	}
	want := `1
00:00:06,000 --> 00:00:08,000
v1 line1
line2

2
00:00:18,000 --> 00:00:20,000
v3 line1
line2

`
	got := b.String()
	if want != got {
		t.Errorf("got: %q, want: %q", got, want)
	}
}

func writeFile(t *testing.T, file string, contents string) {
	f, err := os.Create(file)
	defer f.Close()
	if err != nil {
		t.Fatal(err)
	}
	_, err = fmt.Fprint(f, contents)
	if err != nil {
		t.Fatal(err)
	}
}

func TestHardSubM3U(t *testing.T) {
	dir, err := ioutil.TempDir("", "crkr_whsm")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	videos := []string{}
	for _, name := range []string{"a", "b", "c"} {
		videos = append(videos, filepath.Join(dir, name + ".mp4"))
	}
	for _, f := range videos[:2] {
		writeFile(t, subtitledVideoFilename(f), "")
	}
	playlist := fmt.Sprintf(`#EXTM3U
#EXTINF:-1,Guys be like #superbowl #sexism #relatable
%s
#nosubtitles
#EXTINF:-1,Idiots Assemble!
%s
#EXTINF:-1,My First Vine.
%s
`, videos[0], videos[1], videos[2])
	subbed := []string{}
	for _, f := range videos {
		subbed = append(subbed, subtitledVideoFilename(f))
	}
	b := &bytes.Buffer{}
	err = HardSubM3U(b, strings.NewReader(playlist))
	if err != nil {
		t.Fatal(err)
	}
	want := fmt.Sprintf(`#EXTM3U
#EXTINF:-1,Guys be like #superbowl #sexism #relatable
%s
#nosubtitles
#EXTINF:-1,Idiots Assemble!
%s
#EXTINF:-1,My First Vine.
%s
`, subbed[0], videos[1], videos[2])
	got := b.String()
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
