package main

import (
	"bufio"
	"bytes"
	"encoding/csv"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/corpix/uarand"
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
	maxRedirects     = 10
)

type resource struct {
	url             string
	status          string
	redirectsNumber int
	finalURL        string
	err             error
}

var client = &http.Client{
	Timeout: 1 * time.Minute,
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	},
}

func (r *resource) Request(userAgent *string) {
	r.finalURL = "" //  clean first as it may have URL left from previous 405

	req, err := http.NewRequest(http.MethodHead, r.url, http.NoBody)
	if err != nil {
		r.err = err
		return
	}

	req.Header.Add("User-Agent", *userAgent)

	res, err := client.Do(req)
	if err != nil {
		r.err = err
		return
	}

	for res.StatusCode == 301 || res.StatusCode == 302 {
		r.redirectsNumber++

		if r.redirectsNumber > maxRedirects {
			r.err = errors.New("max redirects number reached")
			r.status = "-1"
			return
		}

		urlObj, err := url.Parse(res.Header.Get("Location"))
		if err != nil {
			r.err = err
			return
		}

		if !urlObj.IsAbs() {
			// originalUrl, _ := url.Parse(r.url)
			// url.Host = originalUrl.Host
			urlObj.Host = res.Request.Host
			urlObj.Scheme = "https"
		}

		req, err := http.NewRequest(http.MethodHead, urlObj.String(), http.NoBody)
		if err != nil {
			r.err = err
			return
		}

		req.Header.Add("User-Agent", *userAgent)

		res, err = client.Do(req)
		if err != nil {
			r.err = err
			return
		}

		if res.StatusCode == 405 {
			break
		}
	}

	if r.redirectsNumber > 0 {
		r.finalURL = res.Request.URL.String()
	}

	r.status = strconv.Itoa(res.StatusCode)

}

func requester(in <-chan *resource, out chan<- *resource, userAgent *string, isRandomUA bool) {
	for r := range in {
		backoff.Retry(func() error {
			ua := userAgent

			if isRandomUA {
				randomUA := uarand.GetRandom()
				ua = &randomUA
			}

			r.Request(ua)
			if r.status == "429" || r.status == "405" {
				return errors.New("429 or 405")
			}

			return nil

		}, &backoff.ExponentialBackOff{
			InitialInterval:     1 * time.Second,
			MaxInterval:         60 * time.Second,
			MaxElapsedTime:      2 * time.Minute,
			RandomizationFactor: 0.5,
			Multiplier:          0.5,
			Stop:                -1,
			Clock:               backoff.SystemClock,
		})

		out <- r
	}
}

func serializer(in <-chan *resource, wg *sync.WaitGroup, writer *csv.Writer) {
	writer.Write([]string{"url", "status", "redirects number", "final URL", "error"})

	for r := range in {
		errString := ""

		if r.err != nil {
			errString = r.err.Error()
		}

		if r.status == "" {
			r.status = "err"
		}

		writer.Write([]string{r.url, r.status, strconv.Itoa(r.redirectsNumber), r.finalURL, errString})

		wg.Done()
	}
}

func parseUrls(urlsFilePath, outputFilePath string, numWorkers int, userAgent *string, isRandomUA bool) {
	var wg sync.WaitGroup
	pending, complete := make(chan *resource), make(chan *resource)

	for i := 0; i < numWorkers; i++ {
		go requester(pending, complete, userAgent, isRandomUA)
	}

	csvFile, err := os.Create(outputFilePath)
	if err != nil {
		panic(err)
	}

	defer csvFile.Close()

	writer := csv.NewWriter(csvFile)

	defer writer.Flush()

	go serializer(complete, &wg, writer)

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
		pending <- &resource{url: scanner.Text()}
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

	flag.Parse()

	// if flag.NFlag() == 0 {
	// 	flag.PrintDefaults()
	// 	return
	// }

	fmt.Printf("Starting at %s\n", time.Now().Format(time.RFC850))
	parseUrls(*urlsFilePath, *outputFilePath, *numWorkers, userAgentString, *isRandomUA)
}
