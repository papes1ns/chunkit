package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
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

const (
	startOfDay = 9            // 9 AM
	endOfDay   = 17           // 5 PM
	dateLayout = "2006-01-02" // YYYY-MM-DD
)

func main() {
	dateStr := flag.String("date", time.Now().Format(dateLayout), "The date in the format 'YYYY-MM-DD'")
	flag.Parse()
	date, err := time.ParseInLocation(dateLayout, *dateStr, time.Now().Location())
	if err != nil {
		log.Fatal(err.Error())
	}

	ctx := context.Background()
	oauth2Client, err := authenticateClient(ctx)
	if err != nil {
		log.Fatalf(err.Error())
	}
	calendarService, err := calendar.NewService(ctx, option.WithHTTPClient(oauth2Client))
	if err != nil {
		log.Fatalf(err.Error())
	}

	result, _ := calendarService.Events.List("primary").
		ShowDeleted(false).
		SingleEvents(true).
		TimeMin(date.Format(time.RFC3339)).
		TimeMax(date.Add(24 * time.Hour).Format(time.RFC3339)).
		OrderBy("startTime").
		Do()

	chunks := Chunkify(date, result.Items)

	totalHours := 0.0
	buf := strings.Builder{}

	buf.WriteString("start,end,notes\n")
	for _, chunk := range chunks {
		totalHours += chunk.end.Sub(chunk.start).Hours()
		line := fmt.Sprintf("%s,%s,%s\n",
			formatTime(chunk.start),
			formatTime(chunk.end),
			chunk.notes,
		)
		buf.WriteString(line)
	}

	output := fmt.Sprintf(`
CSV report for the date: %s with a total of %.2f hours.

%s`,
		date.Format(dateLayout),
		totalHours,
		buf.String(),
	)
	fmt.Print(output)
}

type Chunk struct {
	*calendar.Event
	start time.Time
	end   time.Time
	notes string
}

func Chunkify(date time.Time, items []*calendar.Event) []*Chunk {
	var (
		lo        time.Time = date.Add(startOfDay * time.Hour)
		hi        time.Time = date.Add(endOfDay * time.Hour)
		i         int       = 0
		chunks    []*Chunk  = make([]*Chunk, 0, len(items)*2)
		intersect *Chunk
	)

	if len(items) == 0 {
		chunks = append(chunks, &Chunk{start: lo, end: hi, notes: ""})
		return chunks
	}

	for _, e := range items {
		// exclude all-day events
		if e.Start.DateTime == "" || e.End.DateTime == "" {
			continue
		}

		// include event if you created it and are not an attendee
		if len(e.Attendees) == 0 && e.Creator.Self {
			e.Attendees = append(e.Attendees, &calendar.EventAttendee{
				Self: true,
			})
		}

		for _, attendee := range e.Attendees {
			// exclude events you are not an attendee or declined
			if !attendee.Self || attendee.ResponseStatus == "declined" {
				continue
			}

			start := roundToNearest15(e.Start)
			end := roundToNearest15(e.End)

			// include gap chunk if event starts after start of day
			if start.After(lo) {
				chunks = append(chunks, &Chunk{start: lo, end: start, notes: ""})
				if intersect != nil {
					chunks[len(chunks)-1].notes = intersect.notes
				}
			}

			// include current event chunk and keep track of index
			chunks = append(chunks, &Chunk{Event: e, start: start, end: end, notes: e.Summary})
			i = len(chunks) - 1

			// modify previous chunk if current event intersects
			if i > 0 && start.Before(chunks[i-1].end) {
				intersect = chunks[i-1]
				chunks[i-1].end = start
			}

			lo = chunks[i].end
		}
	}

	// if last event ends before end of day, add a gap chunk
	if lo.Before(hi) {
		chunks = append(chunks, &Chunk{start: lo, end: hi, notes: ""})
		if intersect != nil {
			chunks[len(chunks)-1].notes = intersect.notes
		}
	}

	return chunks
}

func roundToNearest15(dt *calendar.EventDateTime) time.Time {
	t, _ := time.Parse(time.RFC3339, dt.DateTime)
	// 7.5 minutes rounds up to 15 minutes, 7.49 minutes rounds down to 0 minutes
	return t.Round(15 * time.Minute)
}

func formatTime(t time.Time) string {
	// valid hours 00-23
	// valid minutes 00, 25, 50, 75
	// valid time 00:00, 00:15, 00:30, 00:45, 01:00, 01:15, ..., 23:45
	return fmt.Sprintf("%s.%02d", t.Format("15"), int(math.Round(float64(t.Minute())/60*100)))
}

func authenticateClient(ctx context.Context) (*http.Client, error) {
	bytes, err := os.ReadFile("credentials.json")
	if err != nil {
		return nil, fmt.Errorf("error reading the credentials file: %v", err)
	}

	config, err := google.ConfigFromJSON(bytes, "https://www.googleapis.com/auth/calendar.events.readonly")
	if err != nil {
		return nil, fmt.Errorf("error creating the OAuth2 config: %v", err)
	}

	tokFile, err := os.OpenFile("token.json", os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return nil, fmt.Errorf("error opening the token file: %v", err)
	}
	defer tokFile.Close()

	tok := &oauth2.Token{}
	json.NewDecoder(tokFile).Decode(tok)

	if tok.Valid() {
		return config.Client(ctx, tok), nil
	}

	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	fmt.Printf("Authenticate at this URL:\n\n%s\n", authURL)

	ch := make(chan string, 1)
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		ch <- r.URL.Query().Get("code")
		w.Write([]byte("You can now close this window."))
	})

	go http.ListenAndServe(":"+strings.Split(config.RedirectURL, ":")[2], nil)

	tok, _ = config.Exchange(ctx, <-ch)

	// save the token for future use
	tokFile.Seek(0, 0)
	tokFile.Truncate(0)
	json.NewEncoder(tokFile).Encode(tok)

	return config.Client(ctx, tok), nil
}
