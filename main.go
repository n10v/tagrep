// Copyright 2017 Albert Nigmatzianov. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/bogem/id3v2"
	"github.com/spf13/pflag"
)

var (
	// Flag values.
	flagArtist, flagTitle, flagYear                     string
	flagAbs, flagRecursive, flagIgnoreCase, flagVerbose bool
	flagExts                                            []string

	// For internal usage.
	inExts       map[string]bool
	tagPool      = sync.Pool{New: func() interface{} { return id3v2.NewEmptyTag() }}
	total, found int64
	wd           string
)

func main() {
	pflag.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage:
  tagrep [flags] paths

Flags:
`)
		pflag.PrintDefaults()
	}

	pflag.BoolVar(&flagAbs, "abs", false, "print absolute paths")
	pflag.StringVar(&flagArtist, "artist", "", "match artist")
	pflag.StringSliceVarP(&flagExts, "exts", "e", []string{".mp3"}, `parse files only with given extensions. use "*" for parsing all files`)
	pflag.BoolVarP(&flagIgnoreCase, "ignore-case", "i", false, "ignore case on matching frames")
	pflag.BoolVarP(&flagRecursive, "recursive", "r", false, "recursive search")
	pflag.StringVar(&flagTitle, "title", "", "match title")
	pflag.BoolVarP(&flagVerbose, "verbose", "v", false, "verbose output")
	pflag.StringVar(&flagYear, "year", "", "match year")
	pflag.Parse()

	dirs := pflag.Args()
	if len(dirs) == 0 {
		fmt.Println("ERROR: enter at least one path")
		pflag.Usage()
		os.Exit(1)
	}

	if flagAbs {
		var err error
		wd, err = os.Getwd()
		if err != nil {
			log.Fatalln(err)
		}
	}

	initOptions()

	if len(flagExts) > 0 && flagExts[0] != "*" {
		inExts = make(map[string]bool, len(flagExts))
		for _, ext := range flagExts {
			inExts[ext] = true
		}
	}

	var wg sync.WaitGroup
	t := time.Now()
	for _, dir := range dirs {
		wg.Add(1)
		go search(dir, &wg)
	}
	wg.Wait()
	expired := time.Since(t)

	fmt.Printf("%v files total, %v found in %vms\n", total, found, int(1000*expired.Seconds()))
}

var opts = id3v2.Options{
	Parse: true,
}

func initOptions() {
	if flagArtist != "" {
		opts.ParseFrames = append(opts.ParseFrames, "Artist")
	}
	if flagTitle != "" {
		opts.ParseFrames = append(opts.ParseFrames, "Title")
	}
	if flagYear != "" {
		opts.ParseFrames = append(opts.ParseFrames, "Year")
	}
	if len(opts.ParseFrames) == 0 {
		// No frames to parse. Exit.
		os.Exit(0)
	}
}

func search(dir string, wg *sync.WaitGroup) {
	defer wg.Done()

	fileInfos, err := readDir(dir)
	if err != nil {
		log.Fatal(err)
	}

	wg.Add(len(fileInfos))
	for _, fileInfo := range fileInfos {
		go func(fi os.FileInfo) {
			defer wg.Done()

			// Increment total.
			if !fi.IsDir() {
				atomic.AddInt64(&total, 1)
			}

			// Check if file is more than 20 bytes.
			// It makes no sense to parse file less than 20 bytes,
			// because header of ID3v2 tag and of one frame header equal to 20 bytes.
			if fi.Size() < 20 {
				return
			}

			// Path to file.
			path := filepath.Join(dir, fi.Name())

			// If it's dir and recursive flag is set,
			// then parse tracks there, else end the search.
			if fi.IsDir() {
				if flagRecursive {
					wg.Add(1)
					search(path, wg)
				}
				return
			}

			if len(inExts) > 0 && !inExts[filepath.Ext(fi.Name())] {
				return
			}

			match(path)
		}(fileInfo)
	}
}

// Copy of ioutil.ReadDir but just without sort.
func readDir(dirname string) ([]os.FileInfo, error) {
	f, err := os.Open(dirname)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return f.Readdir(-1)
}

func match(path string) {
	// Open file.
	file, err := os.Open(path)
	if err != nil {
		if flagVerbose {
			log.Println("ERROR: ", path, ":", err)
		}
		return
	}
	defer file.Close()

	// Acquire tag from pool and find in file the ID3v2 tag.
	tag := tagPool.Get().(*id3v2.Tag)
	defer tagPool.Put(tag)
	if err := tag.Reset(file, opts); err != nil {
		if flagVerbose {
			log.Println("ERROR: ", path, ":", err)
		}
		return
	}

	if !tag.HasFrames() {
		return
	}

	if flagArtist != "" && !areStringsEqual(tag.Artist(), flagArtist, flagIgnoreCase) {
		return
	}
	if flagTitle != "" && !areStringsEqual(tag.Title(), flagTitle, flagIgnoreCase) {
		return
	}
	if flagYear != "" && !areStringsEqual(tag.Year(), flagYear, flagIgnoreCase) {
		return
	}

	atomic.AddInt64(&found, 1)

	if flagAbs && !filepath.IsAbs(path) {
		fmt.Println(filepath.Join(wd, path))
	} else {
		fmt.Println(path)
	}
}

func areStringsEqual(a, b string, ignoreCase bool) bool {
	if ignoreCase {
		return strings.EqualFold(a, b)
	}
	return a == b
}
