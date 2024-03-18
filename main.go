package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"net/http"
	"os"
	"strings"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"
)

func main() {
	now := time.Now()
	dateStr := flag.String("date", now.Format("2006-01-02"), "The date in the format 'YYYY-MM-DD'")
	flag.Parse()
	date, err := time.ParseInLocation("2006-01-02", *dateStr, now.Location())
	if err != nil {
		fmt.Println("Invalid date format. Please use 'YYYY-MM-DD'", err)
		os.Exit(1)
	}

	ctx := context.Background()
	oauth2Client := getAuthenticatedClient(ctx)
	calendarService, err := calendar.NewService(ctx, option.WithHTTPClient(oauth2Client))
	if err != nil {
		fmt.Println("Error creating the calendar service:", err)
		os.Exit(1)
	}

	result, _ := calendarService.Events.List("primary").
		ShowDeleted(false).
		SingleEvents(true).
		TimeMin(date.Format(time.RFC3339)).
		TimeMax(date.Add(24 * time.Hour).Format(time.RFC3339)).
		OrderBy("startTime").
		Do()

	chunks := makeChunks(date, result.Items)

	// here's an example of how you can print the chunks in a CSV format
	totalHours := 0.0
	buffer := strings.Builder{}

	buffer.WriteString("start,end,notes\n")
	for _, chunk := range chunks {
		totalHours += chunk.end.Sub(chunk.start).Hours()
		line := fmt.Sprintf("%s,%s,%s\n",
			chunk.formatTime(chunk.start),
			chunk.formatTime(chunk.end),
			chunk.notes,
		)
		buffer.WriteString(line)
	}

	output := fmt.Sprintf(`CSV report for the date: %s with a total of %.2f hours.

%s`,
		date.Format("2006-01-02"),
		totalHours,
		buffer.String(),
	)
	fmt.Print(output)
}

type chunk struct {
	*calendar.Event
	start time.Time
	end   time.Time
	notes string
}

func (c *chunk) formatTime(t time.Time) string {
	return fmt.Sprintf("%s.%02d", t.Format("15"), int(math.Round(float64(t.Minute())/60*100)))
}

func makeChunks(date time.Time, items []*calendar.Event) []*chunk {
	var (
		// assuming 9 to 5 work day
		lo        time.Time = date.Add(9 * time.Hour)
		hi        time.Time = date.Add(17 * time.Hour)
		i         int       = 0
		chunks    []*chunk  = make([]*chunk, 0, len(items)*2)
		intersect *chunk
	)

	if len(items) == 0 {
		chunks = append(chunks, &chunk{start: lo, end: hi, notes: ""})
		return chunks
	}

	for _, e := range items {
		// skip all day events
		if e.Start.DateTime == "" || e.End.DateTime == "" {
			continue
		}

		// if no attendees, assume it's your own event
		if len(e.Attendees) == 0 && e.Creator.Self {
			e.Attendees = append(e.Attendees, &calendar.EventAttendee{
				Self: true,
			})
		}

		for _, attendee := range e.Attendees {
			// if you didn't decline the event it counts
			if !attendee.Self || attendee.ResponseStatus == "declined" {
				continue
			}
			start, end, _ := roundToNearest15(e.Start.DateTime, e.End.DateTime)
			// if event start after previous event end, add a gap block
			if start.After(lo) {
				chunks = append(chunks, &chunk{start: lo, end: start, notes: ""})
				if intersect != nil {
					chunks[len(chunks)-1].notes = intersect.notes
				}
			}

			chunks = append(chunks, &chunk{Event: e, start: start, end: end, notes: e.Summary})
			i = len(chunks) - 1

			// check if previous event ends after this event starts
			if i > 0 && start.Before(chunks[i-1].end) {
				intersect = chunks[i-1]
				chunks[i-1].end = start
			}

			lo = chunks[i].end
		}
	}

	// if last event ends before end of day, add a gap block
	if lo.Before(hi) {
		chunks = append(chunks, &chunk{start: lo, end: hi, notes: ""})
		if intersect != nil {
			chunks[len(chunks)-1].notes = intersect.notes
		}
	}

	return chunks
}

func roundToNearest15(times ...string) (time.Time, time.Time, []time.Time) {
	roundedTimes := make([]time.Time, len(times))
	for i, s := range times {
		t, _ := time.Parse(time.RFC3339, s)
		roundedTimes[i] = t.Round(15 * time.Minute)
	}
	return roundedTimes[0], roundedTimes[1], roundedTimes[2:]
}

func getAuthenticatedClient(ctx context.Context) *http.Client {
	bytes, _ := os.ReadFile("credentials.json")
	config, _ := google.ConfigFromJSON(bytes, "https://www.googleapis.com/auth/calendar.events.readonly")

	tok := &oauth2.Token{}
	f, _ := os.OpenFile("token.json", os.O_RDWR|os.O_CREATE, 0644)
	defer f.Close()

	json.NewDecoder(f).Decode(tok)

	if !tok.Valid() {
		authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
		fmt.Printf("Authenticate at this URL:\n\n%s\n", authURL)

		ch := make(chan string, 1)
		srv := &http.Server{
			Addr: ":" + strings.Split(config.RedirectURL, ":")[2],
			Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				ch <- r.URL.Query().Get("code")
				w.Write([]byte("You can now close this window."))
			}),
		}
		defer srv.Shutdown(ctx)

		go srv.ListenAndServe()
		tok, _ = config.Exchange(ctx, <-ch) // block the main thread until the code is received

		// truncate the file and write the new token
		f.Seek(0, 0)
		f.Truncate(0)
		json.NewEncoder(f).Encode(tok)
	}
	return config.Client(ctx, tok)
}
