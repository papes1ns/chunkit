package main

import (
	"testing"
	"time"

	"google.golang.org/api/calendar/v3"
)

func Test_Chunkify(t *testing.T) {
	date := time.Now()
	date = time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, date.Location())

	declinedEvent := newEvent(date.Add(10*time.Hour), date.Add(12*time.Hour), "declined event", "declined", true)
	acceptedEvent := newEvent(date.Add(10*time.Hour), date.Add(12*time.Hour), "accepted event", "accepted", true)
	gapEvent := newEvent(date.Add(13*time.Hour), date.Add(14*time.Hour), "gap event", "accepted", true)
	overlapEvent := newEvent(date.Add(8*time.Hour), date.Add(17*time.Hour), "overlapping event", "accepted", true)

	tests := []struct {
		name          string
		items         []*calendar.Event
		expectedNotes []string
	}{
		{
			name:          "with no calendar events",
			items:         []*calendar.Event{},
			expectedNotes: []string{""},
		},
		{
			name:          "skips declined events",
			items:         []*calendar.Event{declinedEvent},
			expectedNotes: []string{""},
		},
		{
			name:          "includes accepted events",
			items:         []*calendar.Event{acceptedEvent},
			expectedNotes: []string{"", "accepted event", ""},
		},
		{
			name:          "creates gap between events",
			items:         []*calendar.Event{acceptedEvent, gapEvent},
			expectedNotes: []string{"", "accepted event", "", "gap event", ""},
		},
		{
			name:          "handles overlapping event",
			items:         []*calendar.Event{overlapEvent, acceptedEvent, gapEvent},
			expectedNotes: []string{"overlapping event", "accepted event", "overlapping event", "gap event", "overlapping event"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			chunks := Chunkify(date, test.items)

			// check that number of chunks are as expected
			if len(chunks) != len(test.expectedNotes) {
				t.Errorf("expected %d chunks, got %d", len(test.expectedNotes), len(chunks))
			}

			for i, chunk := range chunks {
				// check that notes are as expected
				if chunk.notes != test.expectedNotes[i] {
					t.Errorf("expected chunk notes to be '%s', got '%s'", test.expectedNotes[i], chunk.notes)
				}

				// check that chunks are consecutive
				if i > 0 && chunk.start != chunks[i-1].end {
					t.Errorf("expected chunk %d to start at %s, got %s", i, chunks[i-1].end, chunk.start)
				}
			}
		})
	}
}

func Benchmark_Chunkify(b *testing.B) {
	date := time.Now()
	date = time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, date.Location())

	declinedEvent := newEvent(date.Add(10*time.Hour), date.Add(12*time.Hour), "declined event", "declined", true)
	acceptedEvent := newEvent(date.Add(10*time.Hour), date.Add(12*time.Hour), "accepted event", "accepted", true)
	gapEvent := newEvent(date.Add(13*time.Hour), date.Add(14*time.Hour), "gap event", "accepted", true)
	overlapEvent := newEvent(date.Add(8*time.Hour), date.Add(17*time.Hour), "overlapping event", "accepted", true)
	items := []*calendar.Event{overlapEvent, acceptedEvent, gapEvent, declinedEvent}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Chunkify(date, items)
	}
}

func newEvent(start time.Time, end time.Time, summary string, responseStatus string, self bool) *calendar.Event {
	return &calendar.Event{
		Summary: summary,
		Start:   &calendar.EventDateTime{DateTime: start.Format(time.RFC3339)},
		End:     &calendar.EventDateTime{DateTime: end.Format(time.RFC3339)},
		Attendees: []*calendar.EventAttendee{
			{Self: self, ResponseStatus: responseStatus},
		},
	}
}
