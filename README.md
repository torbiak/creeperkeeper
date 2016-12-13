# Creeper Keeper

Easily create Vine compilations with subtitles generated from metadata.

Uses vine.co's API to get a user's posted and liked videos, and ffmpeg to render the subtitles and combine them all.

## Synopsis

    # Download Vines related to a URL for a single Vine or a user's posts or
    # likes and generate subtitles:
    crkr get URL PLAYLIST [-subformat TEMPLATE] [-t DURATION]

    # Concatenate subtitles for a playlist of videos:
    crkr concatsubs M3U SUBTITLES

    # Render subtitles for a playlist of videos:
    crkr hardsub M3U PLAYLIST [-font sans|Arial] [-fontsize 8]

    # Losslessly concatenate a playlist of MP4 videos:
    crkr concat M3U VIDEO

## Installation

Install [ffmpeg](https://ffmpeg.org/download.html) and ensure the directory containing the `ffmpeg` and `ffprobe` binaries is [added to the PATH environment variable](http://www.howtogeek.com/118594/how-to-edit-your-system-path-for-easy-command-line-access/). `ffmpeg` and `ffprobe` must be in a directory in `PATH` for Creeper Keeper to be able to call them.

Download the [Creeper Keeper binary](TODO).

## Description

Use Creeper Keeper to download Vines and generate subtitles. URLs for single Vines or a user's posts or likes are supported. Vines and subtitle files are named for the Vine's short ID.

    # Produces <UUID>.mp4... <UUID>.srt... miel.m3u
    crkr get https://vine.co/u/973499529959968768 miel.m3u

The subformat option for the get command specifies a Go text template to use for generating subtitles. Available fields are `Title`, `Uploader`, `Venue`, and `Created` (which is a `time.Time`). See the docs for the [text/template](https://golang.org/pkg/text/template/) and [time](https://golang.org/pkg/time/) packages for details. eg: `{{.Uploader}} on {{.Date.Format "2006-01-02"}} at {{.Venue}}: {{.Title}}`

If desired, modify the M3U playlist using a video player or text editor. Many video players will automatically display SubRip subtitles contained in a file having the same name as the playing video file, apart from the file extension.

The subtitles for a playlist can be joined together into a single SubRip file, adjusting the intervals based on the videos' durations. To disable subtitles for a playlist entry include a `# nosubtitles` comment above it; this can be useful for repeating an entry but only displaying subtitles for the first instance.

    crkr concatsubs miel.m3u miel.srt

Soft subtitles are recommended as the video won't need to be re-encoded, preserving quality, and viewers can choose to view with or without subtitles. If required, hard subtitles can be rendered like so:

    # This could take a while.
    # Produces <UUID>.sub.mp4... miel.sub.m3u
    crkr hardsub miel.m3u miel.sub.m3u

Assuming all the videos in a playlist are mp4 they can be losslessly concatenated (ie they will not be transcoded/re-encoded):

    crkr concat miel.m3u miel.mp4
