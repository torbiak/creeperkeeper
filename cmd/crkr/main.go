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

const (
	getUsage = `crkr get [-subformat <template>] [-t <duration>] <url> <playlist> 
  Download vines and generate subtitles and a playlist.`
	concatsubsUsage = `crkr concatsubs <playlist> <subtitles>
  Concatenate subtitles for a playlist of videos.`
	hardsubUsage = `crkr hardsub [-fontsize <size>] <playlist_in> <playlist_out>
  Render subtitles and create a new playlist of subtitled videos.`
	concatUsage = `crkr concat <playlist> <video>
  Losslessly concatenate a playlist of MP4 videos into one video.`
)

var globalFlags, getFlags, concatsubsFlags, hardsubFlags, concatFlags *flag.FlagSet
var helpFlag *bool
var verboseFlag *bool
var templateFlag *string
var durationFlag *int
var fontsizeFlag *int

func init() {
	globalFlags = flag.NewFlagSet("crkr", flag.ContinueOnError)
	// We'll print our own error messages.
	globalFlags.SetOutput(ioutil.Discard)
	helpFlag = globalFlags.Bool("h", false, "show help")
	verboseFlag = globalFlags.Bool("v", false, "increase log verbosity")

	getFlags = flag.NewFlagSet("get", flag.ContinueOnError)
	getFlags.SetOutput(ioutil.Discard)
	templateFlag = getFlags.String("subformat", "{{.Uploader}}{{if .Title}}: {{.Title}}{{end}}", "subtitle template. See README for details.")
	durationFlag = getFlags.Int("t", 2, "subtitle `duration` in seconds.")

	concatsubsFlags = flag.NewFlagSet("concatsubs", flag.ContinueOnError)
	concatsubsFlags.SetOutput(ioutil.Discard)

	hardsubFlags = flag.NewFlagSet("hardsub", flag.ContinueOnError)
	hardsubFlags.SetOutput(ioutil.Discard)
	fontsizeFlag = hardsubFlags.Int("fontsize", 10, "font `size`.")

	concatFlags = flag.NewFlagSet("concat", flag.ContinueOnError)
	concatFlags.SetOutput(ioutil.Discard)
}

func printCmdUsage(w io.Writer, cmdUsage string, flags *flag.FlagSet) {
	fmt.Fprintln(w, cmdUsage)
	flags.SetOutput(w)
	flags.PrintDefaults()
}

func printUsage(w io.Writer) {
	fmt.Fprintln(w, "usage: crkr <global_opts> <command> <opts> <args>")
	fmt.Fprintln(w, "global options:")
	globalFlags.SetOutput(w)
	globalFlags.PrintDefaults()
	fmt.Fprintln(w, "commands:")
	printCmdUsage(w, getUsage, getFlags)
	printCmdUsage(w, concatsubsUsage, concatsubsFlags)
	printCmdUsage(w, hardsubUsage, hardsubFlags)
	printCmdUsage(w, concatUsage, concatFlags)
}

func fatalUsage(cmdUsage string, flags *flag.FlagSet, err error) {
	fmt.Fprintln(os.Stderr, err)
	printCmdUsage(os.Stderr, cmdUsage, flags)
	os.Exit(2)
}

func main() {
	log.SetFlags(log.Lshortfile)

	globalFlags.Parse(os.Args[1:])
	if *helpFlag {
		printUsage(os.Stdout)
		os.Exit(1)
	}
	crkr.Verbose = *verboseFlag

	if len(globalFlags.Args()) == 0 {
		printUsage(os.Stderr)
		os.Exit(2)
	}
	cmd := globalFlags.Args()[0]
	cmdArgs := globalFlags.Args()[1:]

	var err error
	nargsErr := fmt.Errorf("wrong number of arguments")

	switch cmd {
	case "get":
		err = getFlags.Parse(cmdArgs)
		if err != nil {
			fatalUsage(getUsage, getFlags, err)
		}
		if getFlags.NArg() != 2 {
			fatalUsage(getUsage, getFlags, nargsErr)
		}
		url := getFlags.Arg(0)
		playlist := getFlags.Arg(1)
		err = getCmd(url, playlist, *templateFlag, *durationFlag)
	case "concatsubs":
		err = concatsubsFlags.Parse(cmdArgs)
		if err != nil {
			fatalUsage(concatsubsUsage, concatsubsFlags, err)
		}
		if concatsubsFlags.NArg() != 2 {
			fatalUsage(concatsubsUsage, concatsubsFlags, nargsErr)
		}
		playlist := concatsubsFlags.Arg(0)
		subtitles := concatsubsFlags.Arg(1)
		err = concatsubsCmd(playlist, subtitles)
	case "hardsub":
		err = hardsubFlags.Parse(cmdArgs)
		if err != nil {
			fatalUsage(hardsubUsage, hardsubFlags, err)
		}
		if hardsubFlags.NArg() != 2 {
			fatalUsage(hardsubUsage, hardsubFlags, nargsErr)
		}
		playlistIn := hardsubFlags.Arg(0)
		playlistOut := hardsubFlags.Arg(1)
		err = hardsubCmd(playlistIn, playlistOut, *fontsizeFlag)
	case "concat":
		err = concatFlags.Parse(cmdArgs)
		if err != nil {
			fatalUsage(concatUsage, concatFlags, err)
		}
		if concatFlags.NArg() != 2 {
			fatalUsage(concatUsage, concatFlags, nargsErr)
		}
		playlist := concatFlags.Arg(0)
		video := concatFlags.Arg(1)
		err = concatCmd(playlist, video)
	default:
		fmt.Fprintf(os.Stderr, "invalid command: %s\n", cmd)
		printUsage(os.Stderr)
		os.Exit(2)
	}

	if err != nil {
		log.Fatal(err)
	}
}

func getCmd(url, playlist, subFormat string, subDuration int) error {
	tmpl, err := template.New("subtitles").Parse(subFormat)
	if err != nil {
		return err
	}

	vines, err := crkr.ExtractVines(url)
	if err != nil {
		return err
	}
	nerrors := 0
	if err := crkr.DownloadVines(vines); err != nil {
		nerrors++
		log.Print(err)
	}

	dur := time.Duration(subDuration) * time.Second
	if err := crkr.WriteSubtitles(vines, dur, tmpl); err != nil {
		nerrors++
		log.Print(err)
	}
	err = func() (err error) {
		f, err := os.Create(playlist)
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
	}()
	if err != nil {
		nerrors++
		log.Print(err)
	}
	if nerrors > 0 {
		return fmt.Errorf("error getting vines")
	}
	return nil
}

func concatsubsCmd(playlist, subtitles string) (err error) {
	plFile, err := os.Open(playlist)
	if err != nil {
		return err
	}
	defer plFile.Close()
	plItems, err := crkr.ReadM3U(plFile)
	if err != nil {
		return err
	}

	out, err := os.Create(subtitles)
	if err != nil {
		return err
	}
	defer func() {
		if cerr := out.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()
	return crkr.ConcatSubtitles(out, plItems)
}

func hardsubCmd(playlistIn, playlistOut string, fontsize int) (err error) {
	inFile, err := os.Open(playlistIn)
	if err != nil {
		return err
	}
	defer inFile.Close()
	plItems, err := crkr.ReadM3U(inFile)
	if err != nil {
		return err
	}

	filenames := []string{}
	for _, item := range plItems {
		if item.NoSubtitles {
			continue
		}
		filenames = append(filenames, item.Filename)
	}

	err = crkr.RenderAllSubtitles(filenames, fontsize)
	if err != nil {
		log.Println(err)
	}

	outFile, err := os.Create(playlistOut)
	if err != nil {
		return err
	}
	defer func() {
		if cerr := outFile.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()
	_, err = inFile.Seek(0, os.SEEK_SET)
	if err != nil {
		return err
	}
	return crkr.HardSubM3U(outFile, inFile)
}

func concatCmd(playlist, video string) error {
	plFile, err := os.Open(playlist)
	if err != nil {
		return err
	}
	defer plFile.Close()
	plItems, err := crkr.ReadM3U(plFile)
	if err != nil {
		return err
	}
	files := []string{}
	for _, item := range plItems {
		files = append(files, item.Filename)
	}
	return crkr.ConcatVideos(files, video)
}
