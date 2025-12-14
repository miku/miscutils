// webshare serves the current directory on port 3000.
package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"path"
	"strings"

	"github.com/mdp/qrterminal"
)

var (
	port      = flag.Int("p", 3000, "port to listen on")
	directory = flag.String("d", ".", "directory to share")
	qrPrefix  = flag.String("q", "192", "comma or space separated ip addr prefixes to print qr code for")
)

var privateIPBlocks []*net.IPNet

func init() {
	setupPrivateIPBlocks()
}

// parsePrefixes parses comma or space separated prefix values
func parsePrefixes(input string) []string {
	// Replace commas with spaces and split on whitespace
	parts := strings.Fields(strings.ReplaceAll(input, ",", " "))
	return parts
}

func setupPrivateIPBlocks() {
	for _, cidr := range []string{
		"127.0.0.0/8",    // IPv4 loopback
		"10.0.0.0/8",     // RFC1918
		"172.16.0.0/12",  // RFC1918
		"192.168.0.0/16", // RFC1918
		"169.254.0.0/16", // RFC3927 link-local
		"::1/128",        // IPv6 loopback
		"fe80::/10",      // IPv6 link-local
		"fc00::/7",       // IPv6 unique local addr
	} {
		_, block, err := net.ParseCIDR(cidr)
		if err != nil {
			panic(fmt.Errorf("parse error on %q: %v", cidr, err))
		}
		privateIPBlocks = append(privateIPBlocks, block)
	}
}

func isPrivateIP(ip net.IP) bool {
	if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return true
	}
	for _, block := range privateIPBlocks {
		if block.Contains(ip) {
			return true
		}
	}
	return false
}

func loggingHandler(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Println(r.RemoteAddr, r.Method, r.URL.Path)
		fn := path.Join(".", r.URL.Path)
		file, err := os.Open(fn)
		if err == nil {
			defer file.Close()
			fi, err := file.Stat()
			if err == nil {
				log.Printf("%s [%d]", file.Name(), fi.Size())
			}
		}
		h.ServeHTTP(w, r)
	})
}

func main() {
	flag.Parse()
	fs := http.FileServer(http.Dir(*directory))
	http.Handle("/", loggingHandler(fs))
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		log.Fatal(err)
	}
	config := qrterminal.Config{
		Level:     qrterminal.M,
		Writer:    os.Stdout,
		BlackChar: qrterminal.WHITE,
		WhiteChar: qrterminal.BLACK,
		QuietZone: 1,
	}

	// Parse the prefixes from the flag
	prefixes := parsePrefixes(*qrPrefix)

	// Track if any QR codes were generated and find fallback public IP
	var qrGenerated bool
	var fallbackIP net.IP
	var fallbackLink string

	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok {
			if ipnet.IP.To4() != nil {
				mark := "public"
				if isPrivateIP(ipnet.IP) {
					mark = "private"
				}
				link := fmt.Sprintf("http://%s:%d", ipnet.IP.String(), *port)
				log.Printf("%s [%s]", link, mark)

				// Check if IP matches any of the prefixes
				for _, prefix := range prefixes {
					if strings.HasPrefix(ipnet.IP.String(), prefix) {
						qrterminal.GenerateWithConfig(link, config)
						qrGenerated = true
						break // Only generate QR code once per matching IP
					}
				}

				// Store first public IP as potential fallback
				if !isPrivateIP(ipnet.IP) && fallbackIP == nil {
					fallbackIP = ipnet.IP
					fallbackLink = link
				}
			}
		}
	}

	// If no QR code was generated and we have a public IP fallback, use it
	if !qrGenerated && fallbackIP != nil {
		qrterminal.GenerateWithConfig(fallbackLink, config)
	}

	if err := http.ListenAndServe(fmt.Sprintf(":%d", *port), nil); err != nil {
		log.Fatal(err)
	}
}
