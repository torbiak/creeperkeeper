# Creeper Keeper

Easily download Vines and create compilations with subtitles generated from metadata.

Uses vine.co's API to get a user's posted and liked videos, and ffmpeg to render the subtitles and combine them all.

## Synopsis

    # Download Vines related to a URL for a single Vine or a user's posts or
    # likes and write an M3U playlist for them.
    crkr get <url> <m3u_out>

    # Generate SubRip subtitles.
    crkr subtitles [-subformat TEMPLATE] [-t DURATION] <m3u_in>

    # Render/burn subtitles.
    crkr hardsub [-font <name>] [-fontsize <size>] <m3u_in> <m3u_out>

    # Losslessly concatenate a playlist of MP4 videos:
    crkr concat <m3u_in> <video_out>

## Video

For video instructions for installation and use, watch [Creeper Keeper: Windows primer](https://www.youtube.com/watch?v=E8PizK-HQYw) on youtube.

## Installation

### crkr

Download a Creeper Keeper binary from the [releases page](https://github.com/torbiak/creeperkeeper/releases) and [add the directory it's in to the `PATH` environment variable](https://www.java.com/en/download/help/path.xml).

Mac OSX users can change `PATH` by opening `Terminal.app` and running the following, assuming the `crkr` binary was extracted to `$HOME/bin` (ie. `/Users/YOUR_USERNAME/bin`):

     echo 'PATH=$PATH:$HOME/bin' >> ~/.bashrc
     source ~/.bashrc

### ffmpeg

If you want to burn hard subtitles or combine videos you'll need to install ffmpeg with libass support. `ffmpeg` and `ffprobe` must be in a directory in `PATH` for Creeper Keeper to be able to call them.

**Windows:** Install ffmpeg from [zeranoe](https://ffmpeg.org/download.html#build-windows).

**OSX:** [Install ffmpeg using homebrew](https://trac.ffmpeg.org/wiki/CompilationGuide/MacOSX#ffmpegthroughHomebrew).

**Linux:** If the version of ffmpeg supplied by your package manager doesn't have libass support, compile from source and give `configure` the `--with-libass` flag.

## Description

An example workflow:

Use the get command to download Vines. URLs for single Vines or a user's posts or likes are supported. Vines, metadata, and subtitle files are named for the Vine's short ID.

    # Produces <UUID>.mp4... <UUID>.json... miel.m3u
    crkr get https://vine.co/u/973499529959968768 miel.m3u

Generate subtitles. The subformat option for the subtitles command specifies a Go text template to use for generating subtitles. Available fields are `Title`, `Uploader`, `Venue`, and `Created` (which is a `time.Time`). See the docs for the [text/template](https://golang.org/pkg/text/template/) and [time](https://golang.org/pkg/time/) packages for details.

A verbose example:

    {{.Uploader}} on {{.Date.Format "2006-01-02"}} at {{.Venue}}: {{.Title}}

When compiling a creator's Vines it might be nice to only show the uploader if the Vine was a repost:

    {{if ne .Uploader "mielmonster"}}[{{.Uploader}}] {{end}}{{.Title}}

To use the default:

    # Produces <UUID>.srt...
    crkr subtitles miel.m3u

If desired, modify the M3U playlist using a video player or text editing tools. Many video players will automatically display SubRip subtitles contained in a file having the same name as the playing video file, apart from the file extension. The subtitles can also be easily modified.

Render subtitles:

    # This can take a while.
    # Produces <UUID>.sub.mp4... miel.sub.m3u
    crkr hardsub miel.m3u miel.sub.m3u

To concatenate the Vines without re-encoding the video and decreasing quality, all the videos must be the same size. Most Vines are 720x720, but some are only available at 480x480. The `hardsub` and `concat` commands automatically scale videos as necessary before performing their primary functions so an odd-size video can't sneak in and cause glitches.

Finally, join them all together:

    # Produces miel.mp4
    crkr concat miel.sub.m3u miel.mp4

## Emoji

Emoji are heavily used in many Vine descriptions but they are far from being universally supported. If burnt subtitle emoji are all displayed as replacement characters (commonly represented by an empty rectangle glyph), fontconfig probably can't find an installed font containing them. Free emoji fonts with permissive licenses are available, such as Google's [Noto](https://www.google.com/get/noto/) family. If the given font doesn't contain emoji glyphs fontconfig will take glyphs from a font that does---letters might be from Arial but the emoji could be from Segoe UI, for example. If unexpected glyphs are being displayed after emoji, try using the `subtitles` command's `-plainemoji` option to remove variation selectors, which are mainly used to change the color of the preceding emoji and are relatively new (2014) and unsupported. If unwanted glyphs are still appearing, try replacing the emoji with simpler versions manually.

## Bugs and Limitations

* On Windows, due to a limitation of how filepaths are given to ffmpeg's subtitles filter, all filepaths in a playlist for the `hardsub` command must be on the same drive as the working directory.
