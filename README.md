# tagrep

![Screenshot](./screenshot.png)

tagrep is a tiny CLI utlity for finding tracks with the given ID3 frames
(e.g. artist, title or year).

## Installation

    go get -u github.com/bogem/tagrep

## Usage

```
$ tagrep --help
Usage:
  tagrep [flags] paths

Flags:
      --abs             print absolute paths
      --artist string   match artist
  -e, --exts strings    parse files only with given extensions. use "*" for parsing all files (default [.mp3])
  -i, --ignore-case     ignore case on matching frames
  -r, --recursive       recursive search
      --title string    match title
  -v, --verbose         verbose output
      --year string     match year
```
