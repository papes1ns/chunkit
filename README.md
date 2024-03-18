# chunkit

A prototype tool to help log your daily work written in Go.
It pulls your Google Calendar events and fills in the gaps creating consecutive chunks of time.

## Getting stated

First you need to setup the dependencies listed below.

### Dependencies

- install [Go](https://golang.org/dl/)
- setup access to Google Calendar API. Follow this [guide](https://developers.google.com/calendar/api/quickstart/go)
- ensure you save the `credentials.json` file in the root of this project
- the `token.json` file will be created after you run the program for the first time

After you have setup the dependencies and run the program. You should have this file structure:

```
.
├── README.md
├── credentials.json
├── go.mod
├── go.sum
├── main.go
├── main_test.go
└── token.json
```

## Usage

- `go run main.go` to get the chunks for today
- `go run main.go -date 2024-03-15` to get chunks for a specific date
- `go test` to run unit tests
- `go test -bench=.` to run benchmark

## Credits and references

These projects and resources helped me understand how to use Go and the Google Calendar API.

- https://github.com/motemen/gcal-tui
- https://gobyexample.com/
- Rob Pike - watch his talks on YouTube


