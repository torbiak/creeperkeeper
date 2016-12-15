package creeperkeeper

// CreeperKeeper
// ffmpeg -loglevel warning -i 00001.mp4 -vf "subtitles=f=subs.srt:force_style='FontName=MyFont,Fontsize=8'" vine.mp4
// youtube-dl.exe -o '%(autonumber)s.%(ext)s'
// According to Example 3 relative pathnames should work: https://en.wikipedia.org/wiki/M3U#Examples
//curl 'https://vine.co/api/timelines/users/1108673385032228864/likes?page=1&size=1' | jq . | less
// fontconfig seems to have trouble resolving very general names like "sans" on Windows, so use Arial, which has been included with every Windows since Windows 98. On other operating systems "sans" is expected to resolve to something reasonable.

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"text/template"
	"time"
)

const baseURL = "https://vine.co"
const vineDateFormat = "2006-01-02T15:04:05.999999"

var Verbose = false

var fallbackUUIDCount = 0
var uuidMutex = sync.Mutex{}

var userURLRE *regexp.Regexp
var noSubsRE *regexp.Regexp

func init() {
	userURLRE = regexp.MustCompile(`(?:https?://)?vine\.co/(u/)?([^/]+)(/likes)?/?(\?.*)?$`)
	noSubsRE = regexp.MustCompile("^# *nosubtitles")
}

type Vine struct {
	Title    string
	Uploader string
	URL      string
	UUID     string
	Venue    string
	Created  time.Time
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
	return fmt.Sprintf("1\n00:00:00,000 --> %s\n%s", stopTime, subtitle), nil
}

// Extended M3U entry.
func (v Vine) M3UEntry() string {
	// Cram the title onto one line.
	title := v.Title
	title = strings.Replace(title, "\r", "", -1)
	title = strings.Replace(title, "\n", " ", -1)
	return fmt.Sprintf("#EXTINF:-1,%s: %s\n%s", v.Uploader, title, v.VideoFilename())
}

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

func WriteSubtitles(vines []Vine, t time.Duration, tmpl *template.Template) error {
	var nerrors = 0
	for _, vine := range vines {
		subs, err := vine.Subtitles(t, tmpl)
		if err != nil {
			nerrors += 1
			log.Print(err)
			// Maybe we should return on a template.ExecError?
			continue
		}
		err = ioutil.WriteFile(vine.SubtitlesFilename(), []byte(subs), 0666)
		if err != nil {
			nerrors += 1
			log.Print(err)
		}
	}
	if nerrors > 0 {
		return fmt.Errorf("errors rendering subtitles for %d/%d videos", nerrors, len(vines))
	}
	return nil
}

func DownloadVines(vines []Vine) error {
	nerrors := 0
	nerrorsLock := &sync.Mutex{}

	// Producer
	work := make(chan Vine)
	go func() {
		for _, vine := range vines {
			work <- vine
		}
	}()

	// Consumers.
	wg := &sync.WaitGroup{}
	wg.Add(len(vines))
	for i := 0; i < runtime.NumCPU(); i++ {
		go func() {
			for vine := range work {
				if _, err := os.Stat(vine.VideoFilename()); err == nil {
					// Skip videos that have already been downloaded.
					wg.Done()
					continue
				}
				f, err := os.Create(vine.VideoFilename())
				if err != nil {
					wg.Done()
					continue
				}
				err = vine.Download(f)
				if err != nil {
					log.Println(err)
					nerrorsLock.Lock()
					nerrors += 1
					nerrorsLock.Unlock()
				}
				wg.Done()
			}
		}()
	}

	wg.Wait()
	if nerrors > 0 {
		return fmt.Errorf("errors downloading %d/%d vines", nerrors, len(vines))
	}
	return nil
}

func RenderAllSubtitles(filenames []string, fontSize int) error {
	// A proactive check to avoid getting an error for every video.
	_, err := exec.LookPath("ffmpeg")
	if err != nil {
		return fmt.Errorf("ffmpeg not found in PATH")
	}
	nerrors := 0
	nerrorsLock := &sync.Mutex{}

	// Producer
	work := make(chan string)
	go func() {
		for _, base := range filenames {
			work <- base
		}
	}()

	// Consumers.
	wg := &sync.WaitGroup{}
	wg.Add(len(filenames))
	for i := 0; i < runtime.NumCPU(); i++ {
		go func() {
			for videoFile := range work {
				subbed := subtitledVideoFilename(videoFile)
				if _, err := os.Stat(subbed); err == nil {
					// Skip videos that have already been subtitled. To render
					// subtitles again, delete the existing subtitled file.
					wg.Done()
					continue
				}
				err = RenderSubtitles(subbed, videoFile, fontSize)
				if err != nil {
					log.Print(err)
					nerrorsLock.Lock()
					nerrors += 1
					nerrorsLock.Unlock()
				}
				wg.Done()
			}
		}()
	}

	wg.Wait()
	if nerrors > 0 {
		return fmt.Errorf("errors rendering subtitles for %d/%d videos", nerrors, len(filenames))
	}
	return nil
}

//ffmpeg -i input1.mp4 -c copy -bsf:v h264_mp4toannexb -f mpegts intermediate1.ts
//ffmpeg -i input2.mp4 -c copy -bsf:v h264_mp4toannexb -f mpegts intermediate2.ts
//ffmpeg -i "concat:intermediate1.ts|intermediate2.ts" -c copy -bsf:a aac_adtstoasc output.mp4
func ConcatVideos(videoFiles []string, outFile string) error {
	dir, err := ioutil.TempDir("", "crkr")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)

	tsFiles := []string{}
	for _, f := range videoFiles {
		tsFile := filepath.Join(dir, strings.TrimSuffix(f, ".mp4")+".ts")
		tsFiles = append(tsFiles, tsFile)
		err := mp4ToTransportStream(f, tsFile)
		if err != nil {
			return err
		}
	}
	cmd := exec.Command(
		"ffmpeg",
		"-n",
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
		"-f", "mpegts",
		outFile)
	return runCmd(cmd)
}

func formatExitError(cmd *exec.Cmd, err *exec.ExitError) error {
	return fmt.Errorf("%s: %s:\nstderr:\n%s",
			err.Error(), strings.Join(cmd.Args, " "), err.Stderr)
}

// RenderSubtitles overlays subtitles in ${videoFile%.mp4}.srt on file to
// produce ${videoFile%.mp4}.sub.mp4
func RenderSubtitles(outFile string, videoFile string, fontSize int) error {
	basename := strings.TrimSuffix(videoFile, ".mp4")
	subtitles := basename + ".srt"
	subtitledVideo := subtitledVideoFilename(videoFile)
	style := fmt.Sprintf(
		"subtitles=f=%s:force_style='FontName=Arial,Fontsize=%d'",
		subtitles, fontSize)
	cmd := exec.Command(
		"ffmpeg",
		"-n", // Don't overwrite existing output files.
		"-v", "warning",
		"-i", videoFile,
		"-vf", style,
		subtitledVideo)
	return runCmd(cmd)
}

type vineExtractor func(url string) (vines []Vine, err error)

// ExtractVines gets vine metadata related to a url for a single vine, a user
// profile, or a user's likes. API requests are made as necessary to get all of
// a user's posts or likes.
func ExtractVines(url string) (vines []Vine, err error) {
	extractors := []vineExtractor{
		vineExtractor(vineURLToVines),
		vineExtractor(userURLToVines),
	}
	var errors []string
	for _, extractor := range extractors {
		vines, err := extractor(url)
		if err != nil {
			errors = append(errors, err.Error())
			continue
		}
		return vines, nil
	}
	return nil, fmt.Errorf("vine extraction: %s", strings.Join(errors, ", "))
}

// vineURLToVines GETs url, which is expected to the the url for a single vine,
// and extracts the vine's metadata from some JSON embedded in it.
func vineURLToVines(url string) (vines []Vine, err error) {
	matched, err := regexp.MatchString(`https?://(?:www\.)?vine\.co/(?:v|oembed)/(?P<id>\w+)`, url)
	if !matched {
		return nil, fmt.Errorf("url must be for an individual vine: %s", url)
	}
	if err != nil {
		return nil, err
	}
	// There doesn't seem to be an api endpoint for vines, so look in the html.
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return vineHTMLToVines(body)
}

func vineHTMLToVines(html []byte) (vines []Vine, err error) {
	jsonPattern := regexp.MustCompile(`window\.POST_DATA\s*=\s*({.+?});\s*</script>`)
	m := jsonPattern.FindSubmatch(html)
	if len(m) == 0 {
		return nil, fmt.Errorf("no vine metadata found in html")
	}
	var jm jsonMap
	err = json.Unmarshal(m[1], &jm)
	if err != nil {
		return nil, err
	}
	var jvine jsonVine
	for _, jvine = range jm { // Get first value from map.
		break
	}
	for _, jurl := range jvine.VideoURLs {
		if jurl.ID == "original" {
			created, err := time.Parse(vineDateFormat, jvine.Created)
			if err != nil {
				return nil, err
			}
			vines = append(vines, Vine{
				Title:    jvine.Description,
				Uploader: jvine.Username,
				URL:      jurl.VideoURL,
				UUID:     jvine.ShortID,
				Created:  created,
				Venue:    jvine.VenueName,
			})
			return vines, nil
		}
	}
	return nil, nil
}

func userURLToVines(url string) (vines []Vine, err error) {
	userID, err := userURLToUserID(url)
	if err != nil {
		return nil, err
	}
	m := userURLRE.FindStringSubmatch(url)
	if len(m[3]) > 0 {
		vines, err = likedVines(userID)
	} else {
		vines, err = postedVines(userID)
	}
	if err != nil {
		return nil, err
	}
	return vines, nil
}

func userURLToUserID(url string) (string, error) {
	m := userURLRE.FindStringSubmatch(url)
	if len(m) == 0 {
		return "", fmt.Errorf("unrecognized vine user url: %q", url)
	}
	isVanity := len(m[1]) == 0
	if isVanity {
		profileURL := fmt.Sprintf("%s/api/users/profiles/vanity/%s", baseURL, m[2])
		var ur userResult
		err := deserialize(profileURL, &ur)
		if err != nil {
			return "", err
		}
		return fmt.Sprint(ur.Data.UserID), nil
	} else {
		return m[2], nil
	}
}

func likedVines(userID string) (vines []Vine, err error) {
	url := fmt.Sprintf("%s/api/timelines/users/%s/likes", baseURL, userID)
	return timelineVines(url)
}

func postedVines(userID string) (vines []Vine, err error) {
	url := fmt.Sprintf("%s/api/timelines/users/%s", baseURL, userID)
	return timelineVines(url)
}

func timelineVines(url string) (vines []Vine, err error) {
	more := true
	for i := 1; more; i++ {
		urlWithParams := fmt.Sprintf("%s?page=%d&size=100", url, i)
		var pageVines []Vine
		pageVines, more, err = timelinePageVines(urlWithParams)
		if err != nil {
			return nil, err
		}
		vines = append(vines, pageVines...)
	}
	return vines, nil
}

func timelinePageVines(url string) (vines []Vine, more bool, err error) {
	var tr timelineResult
	err = deserialize(url, &tr)
	if err != nil {
		return nil, false, err
	}
	for _, tv := range tr.Data.Records {
		created, err := time.Parse(vineDateFormat, tv.Created)
		if err != nil {
			return nil, false, err
		}
		vines = append(vines, Vine{
			Title:    tv.Description,
			Uploader: tv.Username,
			URL:      tv.VideoURL,
			UUID:     permalinkToUUID(tv.PermalinkURL),
			Created:  created,
			Venue:    tv.VenueName,
		})
	}
	more = tr.Data.NextPage > 0
	return vines, more, nil
}

func deserialize(url string, d interface{}) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	var env jsonVineEnvelope
	err = json.Unmarshal(body, &env)
	if err != nil {
		return fmt.Errorf("unrecognized json: %s", body)
	}
	if !env.Success {
		return fmt.Errorf("GET %q: status %d: %s", url, resp.StatusCode, env.Error)
	}
	err = json.Unmarshal(body, &d)
	if err != nil {
		return err
	}
	return nil
}

func permalinkToUUID(permalink string) string {
	re := regexp.MustCompile(`https://vine\.co/v/([a-zA-Z0-9]+)$`)
	m := re.FindStringSubmatch(permalink)
	if len(m) == 0 {
		uuidMutex.Lock()
		defer uuidMutex.Unlock()
		return fmt.Sprintf("fallbackID%d", fallbackUUIDCount)
	}
	return string(m[1])
}

func subtitledVideoFilename(videoFile string) string {
	return strings.TrimSuffix(videoFile, ".mp4") + ".sub.mp4"
}

func subtitlesFilename(videoFile string) string {
	return strings.TrimSuffix(videoFile, ".mp4") + ".srt"
}

type jsonVineEnvelope struct {
	Success bool
	Error   string
}

// Single-Vine JSON structures
type jsonMap map[string]jsonVine
type jsonVine struct {
	Username    string
	Description string
	ShortID     string
	VideoURLs   []jsonVideoURL
	VenueName   string
	Created     string
}
type jsonVideoURL struct {
	VideoURL string
	ID       string
}

// Timeline (posts/likes) API JSON structures
type timelineResult struct {
	Data    timelineRecords
	Success bool
	Error   string
}
type timelineRecords struct {
	Records  []timelineVine
	NextPage int
}
type timelineVine struct {
	Username     string
	Description  string
	VideoURL     string
	PermalinkURL string
	VenueName    string
	Created      string
}

// User API JSON structures
type userResult struct {
	Data jsonUser
}
type jsonUser struct {
	UserID int64
}

// concatSubtitles concatenates any subtitle files corresponding to videoFiles
// (having the same name but different extensions), adjusting intervals as
// necessary.
func ConcatSubtitles(w io.Writer, playlist []plItem) error {
	start := time.Duration(0)
	i := 0

	for _, e := range playlist {
		dur, err := duration(e.Filename)
		if err != nil {
			return err
		}

		if e.NoSubtitles {
			start += dur
			continue
		}

		subs, err := ReadSubRipFile(e.Filename)
		if err != nil {
			return err
		}
		for _, sub := range subs {
			i++
			sub.index = i
			sub.start += start
			sub.stop += start
			_, err := fmt.Fprint(w, sub.String())
			if err != nil {
				return err
			}
		}
		start += dur
	}
	return nil
}

type Subtitle struct {
	index       int
	start, stop time.Duration
	msg         string
}

func (s Subtitle) String() string {
	return fmt.Sprintf("%d\n%s --> %s\n%s\n", s.index, formatDuration(s.start), formatDuration(s.stop), s.msg)
}

func formatDuration(d time.Duration) string {
	return fmt.Sprintf("%02d:%02d:%02d,%03d", 
			d / time.Hour,
			d / time.Minute % 60,
			d / time.Second % 60,
			d / time.Millisecond % 1000)
}

func ReadSubRipFile(videoFile string) ([]Subtitle, error) {
	f, err := os.Open(subtitlesFilename(videoFile))
	defer f.Close()
	if os.IsNotExist(err) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	subs, err := parseAllSubRip(f)
	if err != nil {
		return nil, err
	}
	return subs, nil
}

func parseAllSubRip(r io.Reader) ([]Subtitle, error) {
	subs := []Subtitle{}
	s := bufio.NewScanner(r)
	for {
		for {
			if !s.Scan() {
				return subs, s.Err()
			}
			if s.Text() != "" {
				break
			}
		}
		sub, err := parseSubRip(s)
		if err != nil {
			return nil, err
		}
		subs = append(subs, sub)
	}
	return subs, s.Err()
}

func parseSubRip(s *bufio.Scanner) (Subtitle, error) {
	integerLine := regexp.MustCompile(`^(\d+)$`)
	intervalLine := regexp.MustCompile(`^(\d\d:\d\d:\d\d,\d\d\d) --> (\d\d:\d\d:\d\d,\d\d\d)$`)

	// index
	sub := Subtitle{}
	m := integerLine.FindStringSubmatch(s.Text())
	if len(m) == 0 {
		return sub, fmt.Errorf("parse SubRip: expected index, got %q", s.Text())
	}
	var err error
	sub.index, err = strconv.Atoi(m[1])
	if err != nil {
		return sub, err
	}

	// interval
	if !s.Scan() {
		return sub, fmt.Errorf("parse SubRip: expected interval, got eof")
	}
	m = intervalLine.FindStringSubmatch(s.Text())
	if len(m) == 0 {
		return sub, fmt.Errorf("parse SubRip: expected interval, got %q", s.Text())
	}
	sub.start, err = parseSubRipDuration(m[1])
	if err != nil {
		return sub, fmt.Errorf("parse SubRip: %s", err)
	}
	sub.stop, err = parseSubRipDuration(m[2])
	if err != nil {
		return sub, fmt.Errorf("parse SubRip: %s", err)
	}

	// msg
	b := &bytes.Buffer{}
	for s.Scan() && s.Text() != "" {
		b.WriteString(s.Text())
		b.WriteString("\n")
	}
	sub.msg = b.String()
	return sub, nil
}

func parseSubRipDuration(subRip string) (time.Duration, error) {
	s := subRip
	s = strings.Replace(s, ":", "h", 1)
	s = strings.Replace(s, ":", "m", 1)
	s = strings.Replace(s, ",", "s", 1)
	s += "ms"
	dur, err := time.ParseDuration(s)
	if err != nil {
		return 0, fmt.Errorf("malformed SubRip duration: %q", subRip)
	}
	return dur, nil
}

func duration(filename string) (time.Duration, error) {
	cmd := exec.Command(
		"ffprobe",
		"-v", "error",
		"-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1",
		filename)
	if Verbose {
		log.Println("run:", strings.Join(cmd.Args, " "))
	}
	out, err := cmd.Output()
	if err != nil {
		return 0, formatExitError(cmd, err.(*exec.ExitError))
	}
	seconds, err := strconv.ParseFloat(strings.TrimSpace(string(out)), 64)
	if err != nil {
		return 0, err
	}
	return time.Duration(seconds * 1e6) * time.Microsecond, nil
}

// HardSubM3U takes an existing M3U playlist as r and replaces videos with
// their hardsub versions, if they exist and the entry isn't preceded by a
// nosubtitles annotation.
func HardSubM3U(w io.Writer, r io.Reader) (err error) {
	s := bufio.NewScanner(r)
	noSubs := false
	write := func(s string) {
		if err != nil {
			return
		}
		_, err = fmt.Fprintln(w, s)
	}
	for s.Scan() && err == nil {
		if !noSubs && noSubsRE.MatchString(s.Text()) {
			noSubs = true
		}
		if strings.HasPrefix(s.Text(), "#") {
			write(s.Text())
			continue
		}
		if noSubs {
			noSubs = false
			write(s.Text())
			continue
		}
		subbed := subtitledVideoFilename(s.Text())
		if !fileExists(subbed) {
			write(s.Text())
			continue
		}
		write(subbed)
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

func runCmd(cmd *exec.Cmd) error {
	cmdLine := strings.Join(cmd.Args, " ")
	if Verbose {
		log.Println("run:", cmdLine)
	}
	b := &bytes.Buffer{}
	cmd.Stderr = b
	err := cmd.Run()
	if err == nil && len(b.String()) > 0 {
		log.Print("exit status 0: ", cmdLine, "\nstderr:\n", b.String())
	}
	if err != nil {
		return fmt.Errorf("%s: %s\nstderr:\n%s", err.Error(), cmdLine, b.String())
	}
	return nil
}
