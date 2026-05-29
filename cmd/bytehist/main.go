// bytehist reads a file and prints a histogram of its byte distribution:
// which byte value occurs how often.
//
// By default it shows the top 15 bytes as a horizontal bar chart on the
// terminal. Flags let you change the count, disable the chart, show the full
// (unabridged) distribution, and emit machine-readable output (csv, tsv, json)
// instead of the visualization.
//
// Usage:
//
//	go run bytehist.go [flags] <file>
//	go run bytehist.go -            # read from stdin
//
// Examples:
//
//	go run bytehist.go file.bin
//	go run bytehist.go -n 30 file.bin
//	go run bytehist.go -a file.bin
//	go run bytehist.go -t json file.bin
//	go run bytehist.go -q -t csv file.bin
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
)

// entry is the count for a single byte value.
type entry struct {
	Byte  byte   `json:"byte"`  // the byte value (0-255)
	Count uint64 `json:"count"` // how often it occurred
	Hex   string `json:"hex"`   // two-digit hex representation, e.g. "0a"
	Glyph string `json:"glyph"` // printable representation for display
}

func main() {
	var (
		top     = flag.Int("n", 15, "show the top N most frequent bytes")
		all     = flag.Bool("a", false, "show the full, unabridged distribution (overrides -top)")
		noBar   = flag.Bool("q", false, "suppress the horizontal bar chart visualization")
		format  = flag.String("t", "", "machine-readable output instead of the chart: csv, tsv, or json")
		barWide = flag.Int("w", 40, "maximum width of the bar chart in characters")
	)
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage: %s [flags] <file>\n\n", os.Args[0])
		fmt.Fprint(flag.CommandLine.Output(), "Print a histogram of the byte distribution of a file. Use \"-\" for stdin.\n\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	if flag.NArg() != 1 {
		flag.Usage()
		os.Exit(2)
	}

	r, name, err := openInput(flag.Arg(0))
	if err != nil {
		fmt.Fprintf(os.Stderr, "bytehist: %v\n", err)
		os.Exit(1)
	}
	defer r.Close()

	counts, total, err := countBytes(r)
	if err != nil {
		fmt.Fprintf(os.Stderr, "bytehist: reading %s: %v\n", name, err)
		os.Exit(1)
	}

	entries := makeEntries(counts)

	// How many to emit: -all wins, otherwise the top N (clamped to what exists).
	n := *top
	if *all || n > len(entries) {
		n = len(entries)
	}
	if n < 0 {
		n = 0
	}
	selected := entries[:n]

	// A chosen machine-readable format suppresses the visualization entirely.
	if *format != "" {
		if err := emitMachine(os.Stdout, *format, selected, total); err != nil {
			fmt.Fprintf(os.Stderr, "bytehist: %v\n", err)
			os.Exit(1)
		}
		return
	}

	emitText(os.Stdout, selected, total, len(entries), !*noBar, *barWide)
}

// openInput opens the named file, or stdin when the name is "-".
func openInput(name string) (io.ReadCloser, string, error) {
	if name == "-" {
		return io.NopCloser(os.Stdin), "<stdin>", nil
	}
	f, err := os.Open(name)
	if err != nil {
		return nil, name, err
	}
	return f, name, nil
}

// countBytes tallies every byte value in r and returns the per-value counts
// plus the grand total of bytes read.
func countBytes(r io.Reader) (counts [256]uint64, total uint64, err error) {
	br := bufio.NewReader(r)
	buf := make([]byte, 64*1024)
	for {
		nr, rerr := br.Read(buf)
		for _, b := range buf[:nr] {
			counts[b]++
		}
		total += uint64(nr)
		if rerr == io.EOF {
			return counts, total, nil
		}
		if rerr != nil {
			return counts, total, rerr
		}
	}
}

// makeEntries turns the raw counts into entries for every byte that actually
// occurred, sorted by count descending (ties broken by byte value ascending).
func makeEntries(counts [256]uint64) []entry {
	entries := make([]entry, 0, 256)
	for v := 0; v < 256; v++ {
		if counts[v] == 0 {
			continue
		}
		b := byte(v)
		entries = append(entries, entry{
			Byte:  b,
			Count: counts[v],
			Hex:   fmt.Sprintf("%02x", v),
			Glyph: glyph(b),
		})
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Count != entries[j].Count {
			return entries[i].Count > entries[j].Count
		}
		return entries[i].Byte < entries[j].Byte
	})
	return entries
}

// glyph returns a short human-friendly label for a byte: the printable ASCII
// character, a named escape for common control bytes, or a hex placeholder.
func glyph(b byte) string {
	switch b {
	case '\t':
		return `\t`
	case '\n':
		return `\n`
	case '\r':
		return `\r`
	case ' ':
		return "SP"
	}
	if b >= 0x21 && b <= 0x7e {
		return string(b)
	}
	return fmt.Sprintf(`\x%02x`, b)
}

// emitText prints the histogram as human-readable text, optionally with a
// horizontal bar chart.
func emitText(w io.Writer, entries []entry, total uint64, distinct int, bar bool, width int) {
	bw := bufio.NewWriter(w)
	defer bw.Flush()

	fmt.Fprintf(bw, "total bytes: %d   distinct values: %d   showing: %d\n\n", total, distinct, len(entries))
	if len(entries) == 0 {
		return
	}

	// The largest count drives the bar scale; pad numeric columns to align.
	maxCount := entries[0].Count
	countW := len(strconv.FormatUint(maxCount, 10))
	if width < 1 {
		width = 1
	}

	for _, e := range entries {
		pct := 0.0
		if total > 0 {
			pct = float64(e.Count) / float64(total) * 100
		}
		fmt.Fprintf(bw, "0x%s %-4s %*d %6.2f%%", e.Hex, e.Glyph, countW, e.Count, pct)
		if bar {
			fmt.Fprintf(bw, "  %s", makeBar(e.Count, maxCount, width))
		}
		fmt.Fprintln(bw)
	}
}

// makeBar renders a proportional bar of block characters, at least one cell
// wide for any non-zero count.
func makeBar(count, max uint64, width int) string {
	if max == 0 {
		return ""
	}
	cells := int(float64(count) / float64(max) * float64(width))
	if cells < 1 && count > 0 {
		cells = 1
	}
	return strings.Repeat("█", cells)
}

// emitMachine writes the selected entries as csv, tsv, or json.
func emitMachine(w io.Writer, format string, entries []entry, total uint64) error {
	bw := bufio.NewWriter(w)
	defer bw.Flush()

	switch strings.ToLower(format) {
	case "csv":
		return emitDelimited(bw, entries, total, ',')
	case "tsv":
		return emitDelimited(bw, entries, total, '\t')
	case "json":
		enc := json.NewEncoder(bw)
		enc.SetIndent("", "  ")
		return enc.Encode(struct {
			Total   uint64  `json:"total"`
			Entries []entry `json:"entries"`
		}{Total: total, Entries: entries})
	default:
		return fmt.Errorf("unknown format %q (want csv, tsv, or json)", format)
	}
}

// emitDelimited writes entries as delimiter-separated rows with a header.
// The percentage is computed against the grand total so it is meaningful even
// when only the top N rows are emitted.
func emitDelimited(w io.Writer, entries []entry, total uint64, sep rune) error {
	cols := func(fields ...string) {
		fmt.Fprintln(w, strings.Join(fields, string(sep)))
	}
	cols("byte", "hex", "glyph", "count", "percent")
	for _, e := range entries {
		pct := 0.0
		if total > 0 {
			pct = float64(e.Count) / float64(total) * 100
		}
		cols(
			strconv.Itoa(int(e.Byte)),
			e.Hex,
			e.Glyph,
			strconv.FormatUint(e.Count, 10),
			strconv.FormatFloat(pct, 'f', 4, 64),
		)
	}
	return nil
}
