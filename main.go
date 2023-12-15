package main

import (
	"bufio"
	"bytes"
	"encoding/csv"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/JekRock/site-url-checker/pkg/requester"
	"github.com/JekRock/site-url-checker/pkg/serializer"

	"github.com/schollz/progressbar/v3"
)

func lineCounter(r io.Reader) (int, error) {
	buf := make([]byte, 32*1024)
	count := 0
	lineSep := []byte{'\n'}

	for {
		c, err := r.Read(buf)
		count += bytes.Count(buf[:c], lineSep)

		switch {
		case err == io.EOF:
			return count, nil

		case err != nil:
			return count, err
		}
	}
}

const (
	userAgentDefault = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10.15; rv:109.0) Gecko/20100101 Firefox/109.0"
)

// getRobotsTxtBody returns the body of robots.txt file
//
// if robotsTxt is a URL, it will be downloaded and returned as string
//
// if robotsTxt is a filesystem path, it will be read and returned as string
func getRobotsTxtBody(robotsTxt string) (string, error) {
	if robotsTxt == "" {
		return "", nil
	}

	robotsTxtURL, err := url.Parse(robotsTxt)
	if err != nil {
		return "", err
	}

	var robotsTxtBody []byte

	if robotsTxtURL.Scheme == "" {
		robotsTxtBody, err = os.ReadFile(robotsTxt)
		if err != nil {
			return "", err
		}
	} else {
		res, err := http.Get(robotsTxt)
		if err != nil {
			return "", err
		}

		robotsTxtBody, err = io.ReadAll(res.Body)
		if err != nil {
			return "", err
		}

	}

	return string(robotsTxtBody), nil
}

func parseUrls(urlsFilePath, outputFilePath string, numWorkers int, userAgent *string, isRandomUA bool, robotsTxt, robotsTxtUserAgent *string) {
	robotsTxtBody, err := getRobotsTxtBody(*robotsTxt)

	if err != nil {
		panic(err)
	}

	var wg sync.WaitGroup
	pending, complete := make(chan *requester.Resource), make(chan *requester.Resource)

	for i := 0; i < numWorkers; i++ {
		go requester.Requester(pending, complete, userAgent, isRandomUA, &robotsTxtBody, robotsTxtUserAgent)
	}

	csvFile, err := os.Create(outputFilePath)
	if err != nil {
		panic(err)
	}

	defer csvFile.Close()

	writer := csv.NewWriter(csvFile)

	defer writer.Flush()

	sz := serializer.CSVSerializer{Writer: writer, Wg: &wg, In: complete}

	go sz.Serialize()

	file, err := os.Open(urlsFilePath)
	if err != nil {
		panic(err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)

	linesCount, err := lineCounter(file)
	if err != nil {
		panic(err)
	}

	bar := progressbar.Default(int64(linesCount))

	file.Seek(0, 0)

	go func() {
		stop := make(chan os.Signal, 1)
		signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
		<-stop
		fmt.Println("[WARN] interrupt signal")

		file.Close()
		writer.Flush()
		csvFile.Close()

		os.Exit(0)
	}()

	for scanner.Scan() {
		wg.Add(1)
		pending <- &requester.Resource{Url: scanner.Text()}
		bar.Add(1)
	}

	fmt.Printf("Before wg.Wait %s\n", time.Now().Format(time.RFC850))
	wg.Wait()
	fmt.Printf("After wg.Wait %s\n", time.Now().Format(time.RFC850))

}

func main() {

	urlsFilePath := flag.String("urls", "urls.txt", "path to file with URLs to check")
	outputFilePath := flag.String("output", "output.csv", "path to output CSV file. If file exists, the content will be overridden")
	numWorkers := flag.Int("numWorkers", 1, "number of parallel workers to make requests")
	userAgentString := flag.String("userAgent", userAgentDefault, "user agent string sent with every request")
	isRandomUA := flag.Bool("randomUserAgent", false, "If set to 'true' every request will have random user agent and 'userAgentString' flag will be ignored")
	robotsTxt := flag.String("robotsTxt", "", "path to robots.txt. Either URL or filesystem path. If set, the script will check if the URL is allowed to be crawled")
	robotsTxtUserAgent := flag.String("robotsTxtUserAgent", userAgentDefault, "user agent string used to validate robots.txt")

	flag.Parse()

	fmt.Printf("Starting at %s\n", time.Now().Format(time.RFC850))
	parseUrls(*urlsFilePath, *outputFilePath, *numWorkers, userAgentString, *isRandomUA, robotsTxt, robotsTxtUserAgent)
}
