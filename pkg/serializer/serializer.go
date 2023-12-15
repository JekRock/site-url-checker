package serializer

import (
	"encoding/csv"
	"strconv"
	"sync"

	"github.com/JekRock/site-url-checker/pkg/requester"
)

// CSVSerializer represents a CSV serializer for the [github.com/JekRock/site-url-checker/pkg/requester.Resource]
type CSVSerializer struct {
	Writer *csv.Writer
	Wg     *sync.WaitGroup
	In     <-chan *requester.Resource
}

// Serialize writes the results to a CSV file
func (s *CSVSerializer) Serialize() {
	_ = s.Writer.Write([]string{"url", "status", "redirects number", "final URL", "allowed by robots.txt", "error"})

	for r := range s.In {
		errString := ""

		if r.Err != nil {
			errString = r.Err.Error()
		}

		if r.Status == "" {
			r.Status = "err"
		}

		_ = s.Writer.Write([]string{r.Url, r.Status, strconv.Itoa(r.RedirectsNumber), r.FinalURL, r.RobotsTxtStatus, errString})

		s.Wg.Done()
	}
}
