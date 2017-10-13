// Copyright 2017 Albert Nigmatzianov. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
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
	artist, title, year                        string
	abs, alpha, recursive, ignoreCase, verbose bool

	// For internal usage.
	total, found int64
	wd           string
	result       = new(bytes.Buffer)
	tagPool      = sync.Pool{New: func() interface{} { return id3v2.NewEmptyTag() }}
)

func main() {
	pflag.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage:
  tagrep [flags] paths

Flags:
`)
		pflag.PrintDefaults()
	}

	pflag.BoolVar(&abs, "abs", false, "print absolute paths")
	pflag.BoolVar(&ignoreCase, "ignore-case", false, "print absolute paths")
	pflag.StringVar(&artist, "artist", "", "match artist")
	pflag.StringVar(&title, "title", "", "match title")
	pflag.BoolVarP(&recursive, "recursive", "r", false, "recursive search")
	pflag.StringVar(&year, "year", "", "match year")
	pflag.BoolVarP(&verbose, "verbose", "v", false, "verbose output")
	pflag.Parse()

	dirs := pflag.Args()
	if len(dirs) == 0 {
		fmt.Println("ERROR: enter at least one path")
		pflag.Usage()
		os.Exit(1)
	}

	var err error
	wd, err = os.Getwd()
	if err != nil {
		log.Fatalln(err)
	}

	initOptions()

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
	if artist != "" {
		opts.ParseFrames = append(opts.ParseFrames, "Artist")
	}
	if title != "" {
		opts.ParseFrames = append(opts.ParseFrames, "Title")
	}
	if year != "" {
		opts.ParseFrames = append(opts.ParseFrames, "Year")
	}
	if len(opts.ParseFrames) == 0 {
		opts.Parse = false
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
			path := joinPaths(dir, fi.Name())

			// If it's dir and recursive flag is set,
			// then parse tracks there, else end the search.
			if fi.IsDir() {
				if recursive {
					wg.Add(1)
					search(path, wg)
				}
				return
			}

			match(path)
		}(fileInfo)
	}
}

// Fast implementation of filepath.Join. It can join only two paths.
// It's about 9-10x faster than filepath.Join and allocates no memory.
func joinPaths(a string, b string) string {
	// If a ends at path separator and b begins with path separator.
	if a[len(a)-1] == os.PathSeparator && b[0] == os.PathSeparator {
		return a + b[1:]
	}
	// If a has no path separator at the end
	// and b has no path separator at the beginning.
	if a[len(a)-1] != os.PathSeparator && b[0] != os.PathSeparator {
		return a + string(os.PathSeparator) + b
	}
	return a + b
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
	// Open file under path.
	file, err := os.Open(path)
	if err != nil {
		if verbose {
			log.Println("ERROR: ", path, ":", err)
		}
		return
	}
	defer file.Close()

	// Acquire tag from pool and find in file the ID3v2 tag.
	tag := tagPool.Get().(*id3v2.Tag)
	defer tagPool.Put(tag)
	if err := tag.Reset(file, opts); err != nil {
		if verbose {
			log.Println("ERROR: ", path, ":", err)
		}
		return
	}

	if !tag.HasFrames() {
		return
	}

	if artist != "" && !areStringsEqual(tag.Artist(), artist, ignoreCase) {
		return
	}
	if title != "" && !areStringsEqual(tag.Title(), title, ignoreCase) {
		return
	}
	if year != "" && !areStringsEqual(tag.Year(), year, ignoreCase) {
		return
	}

	atomic.AddInt64(&found, 1) // found++
	if abs && !filepath.IsAbs(path) {
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
