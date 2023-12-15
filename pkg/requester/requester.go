package requester

import (
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/corpix/uarand"
	"github.com/jimsmart/grobotstxt"
)

const maxRedirects = 10

var client = &http.Client{
	Timeout: 1 * time.Minute,
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	},
}

type Resource struct {
	Url             string
	Status          string
	RedirectsNumber int
	FinalURL        string
	RobotsTxtStatus string
	Err             error
}

func (r *Resource) request(userAgent *string) {
	r.FinalURL = "" //  clean first as it may have URL left from previous 405

	req, err := http.NewRequest(http.MethodHead, r.Url, http.NoBody)
	if err != nil {
		r.Err = err
		return
	}

	req.Header.Add("User-Agent", *userAgent)

	res, err := client.Do(req)
	if err != nil {
		r.Err = err
		return
	}

	for res.StatusCode == 301 || res.StatusCode == 302 {
		r.RedirectsNumber++

		if r.RedirectsNumber > maxRedirects {
			r.Err = errors.New("max redirects number reached")
			r.Status = "-1"
			return
		}

		urlObj, err := url.Parse(res.Header.Get("Location"))
		if err != nil {
			r.Err = err
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
			r.Err = err
			return
		}

		req.Header.Add("User-Agent", *userAgent)

		res, err = client.Do(req)
		if err != nil {
			r.Err = err
			return
		}

		if res.StatusCode == 405 {
			break
		}
	}

	if r.RedirectsNumber > 0 {
		r.FinalURL = res.Request.URL.String()
	}

	r.Status = strconv.Itoa(res.StatusCode)

	if res.Header.Get("x-tncms-bot-tier") != "" {
		r.Err = errors.New("bot-header")
	}

}

func Requester(in <-chan *Resource, out chan<- *Resource, userAgent *string, isRandomUA bool, robotsTxtBody, robotsTxtUserAgent *string) {
	for r := range in {
		backoff.Retry(func() error {
			ua := userAgent

			if isRandomUA {
				randomUA := uarand.GetRandom()
				ua = &randomUA
			}

			r.request(ua)
			if r.Status == "429" || r.Status == "405" {
				return errors.New("429 or 405")
			}

			if *robotsTxtBody != "" {
				allowed := grobotstxt.AgentAllowed(*robotsTxtBody, *robotsTxtUserAgent, r.Url)

				if !allowed {
					r.RobotsTxtStatus = "disallowed"
				} else {
					r.RobotsTxtStatus = "allowed"
				}
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
