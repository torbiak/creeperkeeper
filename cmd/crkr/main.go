package main

import (
	"flag"
	"fmt"
	crkr "github.com/torbiak/creeperkeeper"
	"io"
	"io/ioutil"
	"log"
	"os"
	"text/template"
	"time"
)

var nargsErr = fmt.Errorf("wrong number of arguments")

type Cmd interface {
	PrintUsage(io.Writer)
	Run(args []string)
}

type GetCmd struct {
	flagSet  *flag.FlagSet
	url      string
	playlist string
}

func (c *GetCmd) PrintUsage(w io.Writer) {
	usage := `get <url> <playlist> 
  Download vines and metadata.`
	printCmdUsage(w, usage, c.flags())
}

func (c *GetCmd) flags() *flag.FlagSet {
	if c.flagSet != nil {
		return c.flagSet
	}
	c.flagSet = flag.NewFlagSet("get", flag.ContinueOnError)
	c.flagSet.SetOutput(ioutil.Discard)
	return c.flagSet
}

func (c *GetCmd) Run(args []string) {
	err := c.parseArgs(args)
	if err != nil {
		fatalCmdUsage(c, err)
	}

	vines, err := crkr.ExtractVines(c.url)
	if err != nil {
		log.Fatal(err)
	}

	nerrors := 0

	if err := crkr.WriteAllVineMetadata(vines); err != nil {
		nerrors++
		log.Printf("write metadata: %s", err)
	}

	if err := crkr.DownloadVines(vines); err != nil {
		nerrors++
		log.Printf("download vines: %s", err)
	}

	err = writeM3U(c.playlist, vines)
	if err != nil {
		nerrors++
		log.Printf("write M3U: %s", err)
	}
	if nerrors > 0 {
		log.Fatal("error getting vines")
	}
}

func (c *GetCmd) parseArgs(args []string) error {
	flags := c.flags()
	err := flags.Parse(args)
	if err != nil {
		return err
	}
	if flags.NArg() != 2 {
		return nargsErr
	}
	c.url = flags.Arg(0)
	c.playlist = flags.Arg(1)
	return nil
}

func writeM3U(m3uFile string, vines []crkr.Vine) (err error) {
	f, err := os.Create(m3uFile)
	if err != nil {
		return err
	}
	defer func() {
		if cerr := f.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()
	if err := crkr.WriteM3U(f, vines); err != nil {
		return err
	}
	return nil
}

type SubtitlesCmd struct {
	flagSet    *flag.FlagSet
	plainEmoji bool
	format     string
	duration   float64
	playlist   string
}

func (c *SubtitlesCmd) flags() *flag.FlagSet {
	if c.flagSet != nil {
		return c.flagSet
	}
	c.flagSet = flag.NewFlagSet("subtitles", flag.ContinueOnError)
	c.flagSet.SetOutput(ioutil.Discard)
	c.flagSet.StringVar(&c.format, "format", "[{{.Uploader}}] {{.Title}}", "subtitle template. See README for details.")
	c.flagSet.Float64Var(&c.duration, "t", 2.5, "subtitle `duration` in seconds")
	c.flagSet.BoolVar(&c.plainEmoji, "plainemoji", false, "remove emoji variation selectors")
	return c.flagSet
}

func (c *SubtitlesCmd) PrintUsage(w io.Writer) {
	usage := `subtitles [<opts>] <playlist> 
  Generate SubRip subtitles.`
	printCmdUsage(w, usage, c.flags())
}

func (c *SubtitlesCmd) Run(args []string) {
	err := c.parseArgs(args)
	if err != nil {
		fatalCmdUsage(c, err)
	}

	tmpl, err := template.New("subtitles").Parse(c.format)
	if err != nil {
		log.Fatal(err)
	}

	vines, err := crkr.ReadMetadataForPlaylist(c.playlist)
	if err != nil {
		log.Fatalf("read metadata: %s", err)
	}
	dur := time.Duration(c.duration*1e6) * time.Microsecond
	err = crkr.WriteSubtitles(vines, dur, tmpl, c.plainEmoji)
	if err != nil {
		log.Fatalf("write subtitles: %s", err)
	}
}

func (c *SubtitlesCmd) parseArgs(args []string) error {
	flags := c.flags()
	err := flags.Parse(args)
	if err != nil {
		return err
	}
	if flags.NArg() != 1 {
		return nargsErr
	}
	c.playlist = flags.Arg(0)
	return nil
}

type HardSubCmd struct {
	flagSet       *flag.FlagSet
	font          string
	fontSize      int
	m3uIn, m3uOut string
}

func (c *HardSubCmd) flags() *flag.FlagSet {
	if c.flagSet != nil {
		return c.flagSet
	}
	c.flagSet = flag.NewFlagSet("hardsub", flag.ContinueOnError)
	c.flagSet.SetOutput(ioutil.Discard)
	c.flagSet.IntVar(&c.fontSize, "fontsize", 12, "font `size`")
	c.flagSet.StringVar(&c.font, "font", "Arial", "font `name`")
	return c.flagSet
}

func (c *HardSubCmd) PrintUsage(w io.Writer) {
	usage := `hardsub [<opts>]  <playlist_in> <playlist_out>
  Render subtitles and create a new playlist of subtitled videos.`
	printCmdUsage(w, usage, c.flags())
}

func (c *HardSubCmd) Run(args []string) {
	err := c.parseArgs(args)
	if err != nil {
		fatalCmdUsage(c, err)
	}

	inFile, err := os.Open(c.m3uIn)
	if err != nil {
		log.Fatalf("open playlist: %s", err)
	}
	defer inFile.Close()
	plItems, err := crkr.ReadM3U(inFile)
	if err != nil {
		log.Fatalf("read playlist: %s", err)
	}
	filenames := []string{}
	for _, item := range plItems {
		if item.NoSubtitles {
			continue
		}
		filenames = append(filenames, item.Filename)
	}
	err = crkr.RenderAllSubtitles(filenames, c.font, c.fontSize)
	if err != nil {
		log.Println(err)
	}

	outFile, err := os.Create(c.m3uOut)
	if err != nil {
		log.Fatalf("write playlist: %s", err)
	}
	defer func() {
		if err := outFile.Close(); err != nil {
			log.Printf("write playlist: %s", err)
		}
	}()
	_, err = inFile.Seek(0, os.SEEK_SET)
	if err != nil {
		log.Fatal(err)
	}
	err = crkr.HardSubM3U(outFile, inFile)
	if err != nil {
		log.Fatalf("write playlist: %s", err)
	}
}

func (c *HardSubCmd) parseArgs(args []string) error {
	flags := c.flags()
	err := flags.Parse(args)
	if err != nil {
		return err
	}
	if flags.NArg() != 2 {
		return nargsErr
	}
	c.m3uIn = flags.Arg(0)
	c.m3uOut = flags.Arg(1)
	return nil
}

type ConcatCmd struct {
	flagSet  *flag.FlagSet
	playlist string
	video    string
}

func (c *ConcatCmd) flags() *flag.FlagSet {
	if c.flagSet != nil {
		return c.flagSet
	}
	c.flagSet = flag.NewFlagSet("concat", flag.ContinueOnError)
	c.flagSet.SetOutput(ioutil.Discard)
	return c.flagSet
}

func (c *ConcatCmd) PrintUsage(w io.Writer) {
	usage := `concat <playlist> <video>
  Losslessly concatenate a playlist of MP4 videos into one video.`
	printCmdUsage(w, usage, c.flags())

}

func (c *ConcatCmd) Run(args []string) {
	err := c.parseArgs(args)
	if err != nil {
		fatalCmdUsage(c, err)
	}

	plFile, err := os.Open(c.playlist)
	if err != nil {
		log.Fatalf("open playlist: %s", err)
	}
	defer plFile.Close()
	plItems, err := crkr.ReadM3U(plFile)
	if err != nil {
		log.Fatalf("read playlist: %s", err)
	}
	files := []string{}
	for _, item := range plItems {
		files = append(files, item.Filename)
	}
	err = crkr.ConcatVideos(files, c.video)
	if err != nil {
		log.Fatalf(err.Error())
	}
}

func (c *ConcatCmd) parseArgs(args []string) error {
	flags := c.flags()
	err := flags.Parse(args)
	if err != nil {
		return err
	}
	if flags.NArg() != 2 {
		return nargsErr
	}
	c.playlist = flags.Arg(0)
	c.video = flags.Arg(1)
	return nil
}

func printCmdUsage(w io.Writer, cmdUsage string, flags *flag.FlagSet) {
	fmt.Fprintln(w, cmdUsage)
	flags.SetOutput(w)
	flags.PrintDefaults()
}

func printUsage(w io.Writer, globalFlags *flag.FlagSet, commands map[string]Cmd) {
	fmt.Fprintln(w, "usage: crkr <global_opts> <command> <opts> <args>")
	fmt.Fprintln(w, "\nglobal options:\n")
	globalFlags.SetOutput(w)
	globalFlags.PrintDefaults()
	fmt.Fprintln(w, "\ncommands:\n")
	for _, name := range []string{"get", "subtitles", "hardsub", "concat"} {
		commands[name].PrintUsage(w)
	}
}

func fatalCmdUsage(cmd Cmd, err error) {
	if err != flag.ErrHelp {
		fmt.Fprintln(os.Stderr, err)
	}
	cmd.PrintUsage(os.Stderr)
	os.Exit(2)
}

func main() {
	log.SetFlags(log.Ltime)

	commands := map[string]Cmd{
		"get":       &GetCmd{},
		"subtitles": &SubtitlesCmd{},
		"hardsub":   &HardSubCmd{},
		"concat":    &ConcatCmd{},
	}

	globalFlags := flag.NewFlagSet("crkr", flag.ContinueOnError)
	// We'll print our own error messages.
	globalFlags.SetOutput(ioutil.Discard)
	helpFlag := globalFlags.Bool("h", false, "show help")
	verboseFlag := globalFlags.Bool("v", false, "increase log verbosity")
	err := globalFlags.Parse(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		printUsage(os.Stderr, globalFlags, commands)
		os.Exit(2)
	}

	if *helpFlag {
		printUsage(os.Stdout, globalFlags, commands)
		os.Exit(0)
	}
	crkr.Verbose = *verboseFlag

	if len(globalFlags.Args()) == 0 {
		printUsage(os.Stderr, globalFlags, commands)
		os.Exit(2)
	}

	cmdName := globalFlags.Args()[0]
	cmdArgs := globalFlags.Args()[1:]
	cmd, ok := commands[cmdName]
	if !ok {
		fmt.Fprintf(os.Stderr, "invalid command: %s\n", cmdName)
		printUsage(os.Stderr, globalFlags, commands)
		os.Exit(2)
	}
	cmd.Run(cmdArgs)
}
