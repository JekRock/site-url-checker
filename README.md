# Site URL Checker

> A simple tool to check the HTTP status of a list of URLs.

## Getting started

### Prerequisites and Main Dependencies

- [Golang](https://go.dev/) (1.23+)
- Make

### Installation

To compile the binary, run the following command:

```bash
make build
```

If you need to compile for Linux, run:

```bash
make build-linux
```

### Usage

```bash
$ ./site-url-checker -h
Usage of ./site-url-checker:
  -ignoreRules string
     path to file containing regex ignore rules (one per line)
  -numWorkers int
     number of parallel workers to make requests (default 1)
  -output string
     path to output CSV file. If file exists, the content will be overridden (default "output.csv")
  -randomUserAgent
     If set to 'true' every request will have random user agent and 'userAgentString' flag will be ignored
  -robotsTxt string
     path to robots.txt. Either URL or filesystem path. If set, the script will check if the URL is allowed to be crawled
  -robotsTxtUserAgent string
     user agent string used to validate robots.txt (default "Mozilla/5.0 (Macintosh; Intel Mac OS X 10.15; rv:109.0) Gecko/20100101 Firefox/109.0")
  -urls string
     path to file with URLs to check (default "urls.txt")
  -userAgent string
     user agent string sent with every request (default "Mozilla/5.0 (Macintosh; Intel Mac OS X 10.15; rv:109.0) Gecko/20100101 Firefox/109.0")
```

```bash
$ ./site-url-checker -urls=input.csv -output=output.csv -numWorkers=20 -userAgent="Mozilla/5.0 (Macintosh; Intel Mac OS X 10.15; rv:109.0) Gecko/20100101 Firefox/109.0"
Starting at Thursday, 31-Aug-23 11:07:22 UTC
 100% |███████████████████████████████████████████████████████████████████████████████| (156/156, 19 it/s)
```

Where `input.csv` is a CSV file with the following format:

```csv
https://www.google.com
https://www.facebook.com
https://www.twitter.com
```

## License

Distributed under the MIT license. See ``LICENSE`` for more information.
