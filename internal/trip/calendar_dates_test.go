package trip

import "testing"

func TestConstraintCalendarDates(t *testing.T) {
	h := setup(t, nil)
	saved := h.newSession()
	for _, date := range []string{"0000-01-01", "0000-02-29", "2026-02-29", "2026-1-01", "2026-01-1", "10000-01-01"} {
		for _, end := range []bool{false, true} {
			c := Constraints{StartDate: date}
			if end {
				c = Constraints{EndDate: date}
			}
			h.request("POST", "/api/sessions", sessionInput{Title: "invalid", Constraints: c}, 400, nil)
			h.request("PUT", "/api/sessions/"+saved.ID, sessionInput{Title: "invalid", Constraints: c, Revision: saved.Revision}, 400, nil)
		}
	}
	var sessions []Session
	h.request("GET", "/api/sessions", nil, 200, &sessions)
	if len(sessions) != 1 {
		t.Fatal("invalid dates created sessions")
	}
	var current Session
	h.request("GET", "/api/sessions/"+saved.ID, nil, 200, &current)
	if current.Revision != saved.Revision || current.Constraints != saved.Constraints || current.Title != saved.Title {
		t.Fatal("invalid update changed saved state")
	}
	for _, date := range []string{"", "0001-01-01", "2028-02-29", "9999-12-31"} {
		c := Constraints{StartDate: date, EndDate: date}
		h.request("PUT", "/api/sessions/"+saved.ID, sessionInput{Title: saved.Title, Constraints: c, Revision: current.Revision}, 200, &current)
		if current.Constraints != c {
			t.Fatal("valid calendar date not preserved")
		}
	}
}
