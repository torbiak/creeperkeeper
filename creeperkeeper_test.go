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

var subTemplate = template.Must(template.New("subtitles").Parse("{{.Title}}"))

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
			Title:      "Chicken.",
			Uploader:   "Jack",
			UploaderID: "76",
			URL:        "http://v.cdn.vine.co/v/videos/7508FF74-000E-48F3-9E5E-D7218D5F8FFB-7580-000002B248D861FF_1.1.mp4?versionId=6LYxpZTqmX86aZ9uq5b8e.PqOWi45U9T",
			UUID:       "b9KOOWX7HUx",
			Created:    time.Date(2013, 5, 19, 21, 12, 31, 0, time.UTC),
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
	vines, err := ExtractVines("https://vine.co/u/56")
	if err != nil {
		t.Fatal(err)
	}
	wantLen := 63
	if len(vines) != wantLen {
		t.Errorf("got %d, want %d or more", len(vines), wantLen)
	}
	wantTitles := []string{"le rain.", "spill", "UCB", "ballgame", "kinect"}
	var missing []string
	for _, title := range wantTitles {
		found := false
		for _, vine := range vines {
			if vine.Title == title {
				found = true
			}
		}
		if !found {
			missing = append(missing, title)
		}
	}
	if len(missing) > 0 {
		t.Errorf("missing titles: %s", strings.Join(missing, ", "))
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
	plainEmoji := true
	err := WriteSubtitles(vines, 1*time.Second, subTemplate, plainEmoji)
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
	want := "1\n00:00:00,000 --> 00:00:02,000\nIdiots Assemble!\n"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestReadM3U(t *testing.T) {
	playlist := `#EXTM3U
#EXTINF:-1,Guys be like #superbowl #sexism #relatable
Mz2Wzi73VnI.mp4
#comment
#EXTINF:-1,Idiots Assemble!
bnmHnwVILKD.mp4`
	want := []string{"Mz2Wzi73VnI.mp4", "bnmHnwVILKD.mp4"}
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

	writeBlankVideo(t, videoFile, 720, 720)

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

	err = RenderAllSubtitles([]string{videoFile}, "Arial", 8)
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

func writeBlankVideo(t *testing.T, filename string, width, height int) {
	cmd := exec.Command(
		"ffmpeg",
		"-y",
		"-loglevel", "warning",
		"-t", "6",
		"-f", "lavfi",
		"-i", fmt.Sprintf("color=BLACK:%dx%d", width, height),
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
	dir, err := ioutil.TempDir("", "crkr_concat")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	files := []string{
		filepath.Join(dir, "blank1.mp4"),
		filepath.Join(dir, "blank2.mp4"),
	}
	for _, f := range files {
		writeBlankVideo(t, f, 720, 720)
	}
	outputVideo := filepath.Join(dir, "blank3.mp4")
	err = ConcatVideos(files, outputVideo)
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

func TestHardSubM3U(t *testing.T) {
	dir, err := ioutil.TempDir("", "crkr_whsm")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	videos := []string{}
	for _, name := range []string{"a", "b", "c"} {
		videos = append(videos, filepath.Join(dir, name+".mp4"))
	}
	for _, f := range videos[:2] {
		writeFile(t, SubtitledVideoFilename(f), "")
	}
	playlist := fmt.Sprintf(`#EXTM3U
#EXTINF:-1,Guys be like #superbowl #sexism #relatable
%s
#comment
#EXTINF:-1,Idiots Assemble!
%s
#EXTINF:-1,My First Vine.
%s
`, videos[0], videos[1], videos[2])
	subbed := []string{}
	for _, f := range videos {
		subbed = append(subbed, SubtitledVideoFilename(f))
	}
	b := &bytes.Buffer{}
	err = HardSubM3U(b, strings.NewReader(playlist))
	if err != nil {
		t.Fatal(err)
	}
	want := fmt.Sprintf(`#EXTM3U
#EXTINF:-1,Guys be like #superbowl #sexism #relatable
%s
#comment
#EXTINF:-1,Idiots Assemble!
%s
#EXTINF:-1,My First Vine.
%s
`, subbed[0], subbed[1], videos[2])
	got := b.String()
	if got != want {
		t.Errorf("got %q, want %q", got, want)
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

func TestMetadataCodec(t *testing.T) {
	dir, err := ioutil.TempDir("", "crkr_metacodec")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	err = os.Chdir(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(origDir)

	want := []Vine{
		{
			Title:    "Chicken.",
			Uploader: "Jack",
			URL:      "http://v.cdn.vine.co/v/videos/7508FF74-000E-48F3-9E5E-D7218D5F8FFB-7580-000002B248D861FF_1.1.mp4?versionId=6LYxpZTqmX86aZ9uq5b8e.PqOWi45U9T",
			UUID:     "b9KOOWX7HUx",
			Created:  time.Date(2013, 5, 19, 21, 12, 31, 0, time.UTC),
		},
	}
	err = WriteAllVineMetadata(want)
	if err != nil {
		t.Fatal(err)
	}

	videoFiles := make([]string, len(want))
	for i, v := range want {
		videoFiles[i] = v.VideoFilename()
	}
	playlist := filepath.Join(dir, "pl.m3u")
	err = ioutil.WriteFile(playlist, []byte(strings.Join(videoFiles, "\n")), 0666)
	if err != nil {
		t.Fatal(err)
	}

	got, err := ReadMetadataForPlaylist(playlist)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(want, got) {
		t.Fatalf("want %v, got %v", want, got)
	}
}

func TestVideoDimensions(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping long test")
	}
	dir, err := ioutil.TempDir("", "crkr_tvd")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	video := filepath.Join(dir, "rah.mp4")
	wantW := 720
	wantH := 720
	writeBlankVideo(t, video, wantW, wantH)
	gotW, gotH, err := videoDimensions(video)
	if err != nil {
		t.Fatal(err)
	}
	if wantW != gotW || wantH != gotH {
		t.Fatalf("got %dx%d, want %dx%d", gotW, gotH, wantW, gotH)
	}
}

func TestScale(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping long test")
	}
	dir, err := ioutil.TempDir("", "crkr_scale")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	video := filepath.Join(dir, "orig.mp4")
	writeBlankVideo(t, video, 480, 480)

	err = scale(video)
	if err != nil {
		t.Fatal(err)
	}

	wantW := 720
	wantH := 720
	gotW, gotH, err := videoDimensions(video)
	if err != nil {
		t.Fatal(err)
	}
	if wantW != gotW || wantH != gotH {
		t.Fatalf("got %dx%d, want %dx%d", gotW, gotH, wantW, gotH)
	}
}

func TestScaleAll(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping long test")
	}
	dir, err := ioutil.TempDir("", "crkr_scale")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	a := filepath.Join(dir, "a.mp4")
	b := filepath.Join(dir, "b.mp4")
	writeBlankVideo(t, a, 480, 480)
	writeBlankVideo(t, b, 720, 720)
	videos := []string{a, b}

	err = ScaleAll(videos)
	if err != nil {
		t.Fatal(err)
	}

	wantW := 720
	wantH := 720
	for _, video := range videos {
		gotW, gotH, err := videoDimensions(video)
		if err != nil {
			t.Error(err)
		}
		if wantW != gotW || wantH != gotH {
			t.Errorf("%s: got %dx%d, want %dx%d", video, gotW, gotH, wantW, gotH)
		}
	}
}

func TestRelativePaths(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	abspath := filepath.Join(wd, "bogus1", "bogus2", "bogus3.mp4")
	paths := []string{"random.mp4", abspath}
	want := []string{"random.mp4", "bogus1/bogus2/bogus3.mp4"}
	got, err := relativePaths(paths)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(want, got) {
		t.Fatalf("got %s, want %s", got, want)
	}
}
