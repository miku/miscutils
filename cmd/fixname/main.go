package main

import (
	"crypto/sha1"
	"encoding/hex"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	dryRun    = flag.Bool("d", false, "show what would be renamed without actually renaming")
	recursive = flag.Bool("r", false, "process directories recursively")
	patAlnum  = regexp.MustCompile("[^a-zA-Z0-9_-]")
	patDashes = regexp.MustCompile("[_-]{2,}")
)

func main() {
	flag.Parse()
	args := flag.Args()
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: safename [options] file1 [file2 ...]")
		fmt.Fprintln(os.Stderr, "Options:")
		flag.PrintDefaults()
		return
	}
	for _, path := range args {
		if *recursive {
			err := filepath.Walk(path, func(filePath string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}
				if !info.IsDir() {
					renameToSafe(filePath, *dryRun)
				}
				return nil
			})
			if err != nil {
				fmt.Fprintf(os.Stderr, "error walking directory %s: %v\n", path, err)
			}
		} else {
			renameToSafe(path, *dryRun)
		}
	}
}

func renameToSafe(filePath string, dryRun bool) {
	var (
		dir      = filepath.Dir(filePath)
		filename = filepath.Base(filePath)
		ext      = filepath.Ext(filename)
		name     = strings.TrimSuffix(filename, ext)
		safeName = toSafeName(name)
	)

	// If safe name would be empty, use sha1 hash of the original filename
	if safeName == "" {
		safeName = getSHA1Hash(filename)
	}

	var (
		newFilename = safeName + ext
		newPath     = filepath.Join(dir, newFilename)
	)

	if filename == newFilename {
		return
	}

	fmt.Fprintf(os.Stderr, "renaming: '%s' to '%s'\n", filePath, newPath)
	if !dryRun {
		err := os.Rename(filePath, newPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error renaming '%s': %v\n", filePath, err)
		}
	}
}

func getSHA1Hash(text string) string {
	hash := sha1.New()
	hash.Write([]byte(text))
	return hex.EncodeToString(hash.Sum(nil))
}

func toSafeName(name string) string {
	name = strings.Map(func(r rune) rune {
		if r == ' ' || r == '.' || r == ',' || r == ';' || r == ':' ||
			r == '!' || r == '?' || r == '(' || r == ')' || r == '[' ||
			r == ']' || r == '{' || r == '}' || r == '<' || r == '>' ||
			r == '|' || r == '&' || r == '*' || r == '^' || r == '%' ||
			r == '$' || r == '#' || r == '@' || r == '~' || r == '`' ||
			r == '\'' || r == '"' {
			return '_'
		}
		return r
	}, name)

	name = patAlnum.ReplaceAllString(name, "")
	name = strings.ToLower(name)
	name = patDashes.ReplaceAllString(name, "_")
	return name
}
