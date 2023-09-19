build:
	go build -o site-url-checker
build-linux:
	GOOS=linux GOARCH=amd64 go build -o site-url-checker
