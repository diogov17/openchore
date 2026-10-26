package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/golang-migrate/migrate/v4"
	msqlite "github.com/golang-migrate/migrate/v4/database/sqlite"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/liftedkilt/openchore/internal/model"
	"github.com/liftedkilt/openchore/internal/store"
	"github.com/liftedkilt/openchore/migrations"
	"golang.org/x/oauth2"
	_ "modernc.org/sqlite"
)

func calendarFixture(t *testing.T) (*GoogleCalendarHandler, *model.User) {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:?_foreign_keys=on")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	driver, err := msqlite.WithInstance(db, &msqlite.Config{})
	if err != nil {
		t.Fatal(err)
	}
	src, err := iofs.New(migrations.FS, ".")
	if err != nil {
		t.Fatal(err)
	}
	m, err := migrate.NewWithInstance("iofs", src, "sqlite", driver)
	if err != nil {
		t.Fatal(err)
	}
	if err = m.Up(); err != nil {
		t.Fatal(err)
	}
	s := store.New(db)
	u := &model.User{Name: "Alex", Role: model.RoleAdmin}
	if err = s.CreateUser(context.Background(), u); err != nil {
		t.Fatal(err)
	}
	h := NewGoogleCalendarHandler(s, NewSessionManager([]byte(strings.Repeat("s", 32))))
	h.config.ClientID = "client"
	h.config.ClientSecret = "secret"
	h.config.RedirectURL = "https://private.example/api/google-calendar/callback"
	return h, u
}
func calendarRequest(h http.HandlerFunc, u *model.User, method, path, body string, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r = r.WithContext(context.WithValue(r.Context(), userContextKey, u))
	for _, c := range cookies {
		r.AddCookie(c)
	}
	w := httptest.NewRecorder()
	h(w, r)
	return w
}
func ptr[T any](v T) *T { return &v }

func TestCalendarEventDatesAndRecurrence(t *testing.T) {
	now := time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		name        string
		s           model.ChoreSchedule
		start, rule string
		timed       bool
	}{
		{name: "one off all day", s: model.ChoreSchedule{SpecificDate: ptr("2026-10-25")}, start: "2026-10-25"},
		{name: "weekly", s: model.ChoreSchedule{DayOfWeek: ptr(1)}, start: "2026-10-26", rule: "RRULE:FREQ=WEEKLY"},
		{name: "interval", s: model.ChoreSchedule{RecurrenceInterval: ptr(3), RecurrenceStart: ptr("2026-10-23")}, start: "2026-10-23", rule: "RRULE:FREQ=DAILY;INTERVAL=3"},
		{name: "timed winter", s: model.ChoreSchedule{SpecificDate: ptr("2026-10-26"), AvailableAt: ptr("09:30")}, start: "2026-10-26T09:30:00Z", timed: true},
		{name: "timed summer", s: model.ChoreSchedule{SpecificDate: ptr("2026-10-23"), AvailableAt: ptr("09:30")}, start: "2026-10-23T09:30:00+01:00", timed: true},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			event, err := calendarEvent(&model.Chore{Title: "Clean", EstimatedMinutes: ptr(45)}, tt.s, "Europe/Lisbon", now)
			if err != nil {
				t.Fatal(err)
			}
			start := event["start"].(map[string]string)
			key := "date"
			if tt.timed {
				key = "dateTime"
			}
			if start[key] != tt.start {
				t.Fatalf("start %v, want %s", start, tt.start)
			}
			if tt.rule != "" && event["recurrence"].([]string)[0] != tt.rule {
				t.Fatal(event)
			}
			if tt.timed {
				end, _ := time.Parse(time.RFC3339, event["end"].(map[string]string)["dateTime"])
				begin, _ := time.Parse(time.RFC3339, start[key])
				if end.Sub(begin) != 45*time.Minute {
					t.Fatal(event)
				}
			}
		})
	}
	if _, err := calendarEvent(&model.Chore{}, model.ChoreSchedule{}, "wrong/timezone", now); err == nil {
		t.Fatal("accepted invalid timezone")
	}
}

func TestCalendarCredentialEncryptionAndIsolation(t *testing.T) {
	h, u := calendarFixture(t)
	ctx := context.Background()
	token := &oauth2.Token{AccessToken: "access-secret", RefreshToken: "refresh-secret", Expiry: time.Now().Add(time.Hour)}
	if err := h.save(ctx, u.ID, token, "primary"); err != nil {
		t.Fatal(err)
	}
	c, err := h.store.CalendarConnection(ctx, u.ID)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(c.Token), "secret") {
		t.Fatal("credentials stored in plaintext")
	}
	c.UserID++
	if _, err = h.decrypt(c); err == nil {
		t.Fatal("credential ciphertext can be transferred between profiles")
	}
	other := *u
	other.ID++
	w := calendarRequest(h.Status, &other, "GET", "/api/google-calendar", "")
	if !strings.Contains(w.Body.String(), `"connected":false`) {
		t.Fatal(w.Body.String())
	}
	calendarRequest(h.Disconnect, &other, "DELETE", "/api/google-calendar", "")
	if c, _ := h.store.CalendarConnection(ctx, u.ID); c == nil {
		t.Fatal("another user disconnected this account")
	}
}

func TestCalendarRefreshAndFailureStatus(t *testing.T) {
	h, u := calendarFixture(t)
	ctx := context.Background()
	refreshed := 0
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/token" {
			refreshed++
			r.ParseForm()
			if r.Form.Get("refresh_token") != "refresh" {
				t.Error("wrong refresh token")
			}
			fmt.Fprint(w, `{"access_token":"new-access","expires_in":3600,"token_type":"Bearer"}`)
			return
		}
		w.WriteHeader(403)
	}))
	defer fake.Close()
	h.config.Endpoint.TokenURL = fake.URL + "/token"
	h.config.Endpoint.AuthStyle = oauth2.AuthStyleInParams
	h.apiURL = fake.URL
	if err := h.save(ctx, u.ID, &oauth2.Token{AccessToken: "expired", RefreshToken: "refresh", Expiry: time.Now().Add(-time.Hour)}, "primary"); err != nil {
		t.Fatal(err)
	}
	c, _ := h.store.CalendarConnection(ctx, u.ID)
	token, err := h.token(ctx, c)
	if err != nil {
		t.Fatal(err)
	}
	if refreshed != 1 || token.AccessToken != "new-access" {
		t.Fatal("token was not refreshed")
	}
	c, _ = h.store.CalendarConnection(ctx, u.ID)
	token, err = h.decrypt(c)
	if err != nil || token.RefreshToken != "refresh" || token.AccessToken != "new-access" {
		t.Fatal("refreshed credentials not persisted")
	}
	w := calendarRequest(h.Status, u, "GET", "/api/google-calendar", "")
	if !strings.Contains(w.Body.String(), `"calendars":[]`) || !strings.Contains(w.Body.String(), `"error"`) {
		t.Fatal(w.Body.String())
	}
	if strings.Contains(w.Body.String(), "new-access") {
		t.Fatal("status exposed credentials")
	}
}

func TestCalendarOAuthPKCEAndStateBinding(t *testing.T) {
	h, u := calendarFixture(t)
	exchanged := 0
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		exchanged++
		if err := r.ParseForm(); err != nil {
			t.Error(err)
		}
		if r.Form.Get("code_verifier") == "" {
			t.Error("missing PKCE verifier")
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"access_token":"access","refresh_token":"refresh","expires_in":3600,"token_type":"Bearer"}`)
	}))
	defer fake.Close()
	h.config.Endpoint.TokenURL = fake.URL
	h.config.Endpoint.AuthStyle = oauth2.AuthStyleInParams
	begin := func() (string, *http.Cookie) {
		w := calendarRequest(h.Connect, u, "POST", "/api/google-calendar/connect", "")
		var body map[string]string
		json.Unmarshal(w.Body.Bytes(), &body)
		auth, err := url.Parse(body["url"])
		if err != nil {
			t.Fatal(err)
		}
		q := auth.Query()
		if q.Get("code_challenge_method") != "S256" || q.Get("access_type") != "offline" {
			t.Fatal(q)
		}
		return q.Get("state"), w.Result().Cookies()[0]
	}
	state, cookie := begin()
	w := calendarRequest(h.Callback, u, "GET", "/api/google-calendar/callback?code=test&state=wrong", "", cookie)
	if w.Header().Get("Location") != "/?calendar=failed" || exchanged != 0 {
		t.Fatal("invalid state was accepted")
	}
	other := *u
	other.ID++
	calendarRequest(h.Callback, &other, "GET", "/api/google-calendar/callback?code=test&state="+state, "", cookie)
	if exchanged != 0 {
		t.Fatal("callback linked a different profile")
	}
	state, cookie = begin()
	w = calendarRequest(h.Callback, u, "GET", "/api/google-calendar/callback?code=test&state="+state, "", cookie)
	if w.Header().Get("Location") != "/?calendar=connected" || exchanged != 1 {
		t.Fatal(w.Body.String(), w.Header())
	}
	calendarRequest(h.Callback, u, "GET", "/api/google-calendar/callback?code=test&state="+state, "", cookie)
	if exchanged != 1 {
		t.Fatal("authorization flow replayed")
	}
}

func TestCalendarEventRetryAndWritableSelection(t *testing.T) {
	h, u := calendarFixture(t)
	ctx := context.Background()
	h.save(ctx, u.ID, &oauth2.Token{AccessToken: "access", RefreshToken: "refresh", Expiry: time.Now().Add(time.Hour)}, "primary")
	chore := &model.Chore{Title: "Clean", Category: model.CategoryIndependent, CreatedBy: u.ID}
	if err := h.store.CreateChore(ctx, chore); err != nil {
		t.Fatal(err)
	}
	schedule := &model.ChoreSchedule{ChoreID: chore.ID, AssignedTo: u.ID, AssignmentType: "individual", SpecificDate: ptr("2026-10-23"), PointsMultiplier: 1}
	if err := h.store.CreateSchedule(ctx, schedule); err != nil {
		t.Fatal(err)
	}
	ids := map[string]bool{}
	inserted := 0
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Header.Get("Authorization") != "Bearer access" {
			t.Error("missing authorization")
		}
		if r.Method == "GET" {
			fmt.Fprint(w, `{"items":[{"id":"personal","summary":"Personal","accessRole":"owner","primary":true},{"id":"read-only","summary":"Read only","accessRole":"reader"}]}`)
			return
		}
		var event map[string]any
		json.NewDecoder(r.Body).Decode(&event)
		id := event["id"].(string)
		if ids[id] {
			w.WriteHeader(409)
			return
		}
		ids[id] = true
		inserted++
		w.WriteHeader(201)
	}))
	defer fake.Close()
	h.apiURL = fake.URL
	body := fmt.Sprintf(`{"chore_id":%d,"timezone":"Europe/Lisbon"}`, chore.ID)
	for range 2 {
		w := calendarRequest(h.AddEvents, u, "POST", "/api/google-calendar/events", body)
		if w.Code != 200 {
			t.Fatal(w.Code, w.Body.String())
		}
	}
	if inserted != 1 {
		t.Fatalf("created %d events", inserted)
	}
	w := calendarRequest(h.Select, u, "PUT", "/api/google-calendar", `{"calendar_id":"read-only"}`)
	if w.Code != 400 {
		t.Fatal("accepted read-only calendar")
	}
	w = calendarRequest(h.Select, u, "PUT", "/api/google-calendar", `{"calendar_id":"personal"}`)
	if w.Code != 204 {
		t.Fatal(w.Body.String())
	}
	// A child without an assignment cannot export someone else's chore.
	other := *u
	other.ID++
	other.Role = model.RoleChild
	w = calendarRequest(h.AddEvents, &other, "POST", "/api/google-calendar/events", body)
	if w.Code != 400 {
		t.Fatal(w.Code)
	}
}

// Fake Google records ownership, payloads, deleted IDs and temporary outages.
type calendarGateway struct {
	events             map[string]map[string]any
	deleted            map[string]bool
	failDelete         bool
	failInsertResponse bool
	inserts, patches   int
}

func calendarFake(t *testing.T, h *GoogleCalendarHandler) *calendarGateway {
	t.Helper()
	g := &calendarGateway{events: map[string]map[string]any{}, deleted: map[string]bool{}}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		owner := r.Header.Get("Authorization") + ":"
		path := r.URL.Path
		switch r.Method {
		case "POST":
			var e map[string]any
			if err := json.NewDecoder(r.Body).Decode(&e); err != nil {
				t.Error(err)
				w.WriteHeader(400)
				return
			}
			id := e["id"].(string)
			key := owner + path + "/" + id
			if g.deleted[key] {
				w.WriteHeader(410)
				return
			}
			if g.events[key] != nil {
				w.WriteHeader(409)
				return
			}
			g.events[key] = e
			g.inserts++
			if g.failInsertResponse {
				w.WriteHeader(503)
				return
			}
			w.WriteHeader(201)
		case "PATCH":
			key := owner + path
			if g.events[key] == nil {
				w.WriteHeader(404)
				return
			}
			var e map[string]any
			json.NewDecoder(r.Body).Decode(&e)
			g.events[key] = e
			g.patches++
			w.WriteHeader(200)
		case "DELETE":
			if g.failDelete {
				w.WriteHeader(503)
				return
			}
			key := owner + path
			if g.events[key] == nil {
				w.WriteHeader(410)
				return
			}
			delete(g.events, key)
			g.deleted[key] = true
			w.WriteHeader(204)
		default:
			w.WriteHeader(400)
		}
	}))
	t.Cleanup(server.Close)
	h.apiURL = server.URL
	return g
}
func calendarPerson(t *testing.T, h *GoogleCalendarHandler, name, role string, connected bool) *model.User {
	t.Helper()
	u := &model.User{Name: name, Role: role}
	if err := h.store.CreateUser(context.Background(), u); err != nil {
		t.Fatal(err)
	}
	if connected {
		calendarConnectPerson(t, h, u)
	}
	return u
}
func calendarConnectPerson(t *testing.T, h *GoogleCalendarHandler, u *model.User) {
	t.Helper()
	if err := h.save(context.Background(), u.ID, &oauth2.Token{AccessToken: u.Name, RefreshToken: "refresh-" + u.Name, Expiry: time.Now().Add(time.Hour)}, "calendar-"+u.Name); err != nil {
		t.Fatal(err)
	}
}
func calendarChoreFor(t *testing.T, h *GoogleCalendarHandler, creator *model.User, people ...*model.User) (*model.Chore, []*model.ChoreSchedule) {
	t.Helper()
	ctx := context.Background()
	chore := &model.Chore{Title: "Clean shelves", Category: model.CategoryIndependent, CreatedBy: creator.ID}
	if err := h.store.CreateChore(ctx, chore); err != nil {
		t.Fatal(err)
	}
	schedules := []*model.ChoreSchedule{}
	for _, u := range people {
		s := &model.ChoreSchedule{ChoreID: chore.ID, AssignedTo: u.ID, AssignmentType: "individual", SpecificDate: ptr("2026-10-23"), PointsMultiplier: 1}
		if err := h.store.CreateSchedule(ctx, s); err != nil {
			t.Fatal(err)
		}
		schedules = append(schedules, s)
	}
	return chore, schedules
}
func enableCalendarFor(t *testing.T, h *GoogleCalendarHandler, caller *model.User, choreID int64, want int) {
	t.Helper()
	w := calendarRequest(h.AddEvents, caller, "POST", "/api/google-calendar/events", fmt.Sprintf(`{"chore_id":%d,"timezone":"Europe/Lisbon"}`, choreID))
	if w.Code != want {
		t.Fatal(w.Code, w.Body.String())
	}
}

func TestCalendarUsesEachAssigneeNotCreator(t *testing.T) {
	h, creator := calendarFixture(t)
	g := calendarFake(t, h)
	a := calendarPerson(t, h, "Alice", model.RoleChild, true)
	b := calendarPerson(t, h, "Ben", model.RoleChild, true)
	chore, schedules := calendarChoreFor(t, h, creator, a, b)
	enableCalendarFor(t, h, creator, chore.ID, 200)
	if len(g.events) != 2 {
		t.Fatal("expected one event per assignee", g.events)
	}
	for key := range g.events {
		if !strings.HasPrefix(key, "Bearer Alice:/calendars/calendar-Alice/") && !strings.HasPrefix(key, "Bearer Ben:/calendars/calendar-Ben/") {
			t.Fatal("used creator's calendar", key)
		}
	}
	// A child cannot turn another person's chore into writes to other accounts.
	enableCalendarFor(t, h, a, chore.ID, 403)
	// Removing a schedule through the actual handler deletes only its owner's event.
	chores := &ChoreHandler{store: h.store, calendar: h}
	r := httptest.NewRequest("DELETE", "/api/chores/1/schedules/1", nil)
	route := chi.NewRouteContext()
	route.URLParams.Add("scheduleID", strconv.FormatInt(schedules[0].ID, 10))
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, route))
	w := httptest.NewRecorder()
	chores.DeleteSchedule(w, r)
	if w.Code != 204 {
		t.Fatal(w.Code, w.Body.String())
	}
	if len(g.events) != 1 {
		t.Fatal(g.events)
	}
	for key := range g.events {
		if !strings.HasPrefix(key, "Bearer Ben:") {
			t.Fatal("removed the wrong person's event")
		}
	}
	if g.inserts != 2 {
		t.Fatal("unaffected assignee's event was recreated")
	}
}

func TestCalendarReassignmentAndReturnUsesNewEventID(t *testing.T) {
	h, creator := calendarFixture(t)
	g := calendarFake(t, h)
	ctx := context.Background()
	a := calendarPerson(t, h, "Alice", model.RoleChild, true)
	b := calendarPerson(t, h, "Ben", model.RoleChild, true)
	chore, schedules := calendarChoreFor(t, h, creator, a)
	enableCalendarFor(t, h, creator, chore.ID, 200)
	for _, u := range []*model.User{b, a} {
		schedules[0].AssignedTo = u.ID
		if err := h.store.UpdateSchedule(ctx, schedules[0]); err != nil {
			t.Fatal(err)
		}
		if _, err := h.SyncChore(ctx, chore.ID); err != nil {
			t.Fatal(err)
		}
		if len(g.events) != 1 {
			t.Fatal(g.events)
		}
		for key := range g.events {
			if !strings.HasPrefix(key, "Bearer "+u.Name+":") {
				t.Fatal("event not moved to new assignee", key)
			}
		}
	}
	if g.inserts != 3 {
		t.Fatal("reassignment reused a deleted Google event ID")
	}
	chore.Title = "Clean all shelves"
	if err := h.store.UpdateChore(ctx, chore); err != nil {
		t.Fatal(err)
	}
	if _, err := h.SyncChore(ctx, chore.ID); err != nil {
		t.Fatal(err)
	}
	if g.patches != 1 {
		t.Fatal("existing event not updated")
	}
	for _, event := range g.events {
		if event["summary"] != "Clean all shelves" {
			t.Fatal(event)
		}
	}
}

func TestCalendarRemovalRetriesAfterRestart(t *testing.T) {
	h, creator := calendarFixture(t)
	g := calendarFake(t, h)
	ctx := context.Background()
	a := calendarPerson(t, h, "Alice", model.RoleChild, true)
	chore, schedules := calendarChoreFor(t, h, creator, a)
	enableCalendarFor(t, h, creator, chore.ID, 200)
	g.failDelete = true
	if err := h.store.DeleteSchedule(ctx, schedules[0].ID); err != nil {
		t.Fatal(err)
	}
	if _, err := h.SyncChore(ctx, chore.ID); err == nil {
		t.Fatal("outage ignored")
	}
	entries, _ := h.store.ChoreCalendarEvents(ctx, chore.ID)
	if len(entries) != 1 {
		t.Fatal("lost pending deletion")
	}
	restarted := NewGoogleCalendarHandler(h.store, NewSessionManager([]byte(strings.Repeat("s", 32))))
	restarted.config = h.config
	restarted.apiURL = h.apiURL
	g.failDelete = false
	if _, err := restarted.SyncChore(ctx, chore.ID); err != nil {
		t.Fatal(err)
	}
	if len(g.events) != 0 {
		t.Fatal("event survived assignment removal", g.events)
	}
	entries, _ = h.store.ChoreCalendarEvents(ctx, chore.ID)
	if len(entries) != 0 {
		t.Fatal("deletion was not acknowledged")
	}
}

func TestCalendarWaitsForAssigneesConnection(t *testing.T) {
	h, creator := calendarFixture(t)
	g := calendarFake(t, h)
	a := calendarPerson(t, h, "Alice", model.RoleChild, false)
	chore, _ := calendarChoreFor(t, h, creator, a)
	enableCalendarFor(t, h, creator, chore.ID, 502)
	if len(g.events) != 0 {
		t.Fatal("fell back to creator")
	}
	config, _ := h.store.ChoreCalendar(context.Background(), chore.ID)
	if config == nil {
		t.Fatal("opt-in lost")
	}
	calendarConnectPerson(t, h, a)
	if _, err := h.SyncChore(context.Background(), chore.ID); err != nil {
		t.Fatal(err)
	}
	if len(g.events) != 1 {
		t.Fatal("did not add event after connection")
	}
}

func TestCalendarLostInsertResponseDoesNotDuplicateEvent(t *testing.T) {
	h, creator := calendarFixture(t)
	g := calendarFake(t, h)
	a := calendarPerson(t, h, "Alice", model.RoleChild, true)
	chore, _ := calendarChoreFor(t, h, creator, a)
	g.failInsertResponse = true
	enableCalendarFor(t, h, creator, chore.ID, 502)
	entries, _ := h.store.ChoreCalendarEvents(context.Background(), chore.ID)
	if len(entries) != 1 || entries[0].Payload != "" {
		t.Fatal("insertion was not durably tracked before request")
	}
	g.failInsertResponse = false
	enableCalendarFor(t, h, creator, chore.ID, 200)
	if g.inserts != 1 || len(g.events) != 1 || g.patches != 1 {
		t.Fatal("retry duplicated or lost the event", g.inserts, g.events)
	}
}

func TestCalendarSharedAssignmentsUseAdultsOnly(t *testing.T) {
	h, creator := calendarFixture(t)
	g := calendarFake(t, h)
	ctx := context.Background()
	calendarConnectPerson(t, h, creator)
	adult := calendarPerson(t, h, "Adult", model.RoleAdmin, true)
	calendarPerson(t, h, "Child", model.RoleChild, true)
	chore, schedules := calendarChoreFor(t, h, creator, creator)
	schedules[0].AssignmentType = model.AssignmentFamily
	if err := h.store.UpdateSchedule(ctx, schedules[0]); err != nil {
		t.Fatal(err)
	}
	enableCalendarFor(t, h, creator, chore.ID, 200)
	if len(g.events) != 2 {
		t.Fatal("shared chore did not reach both adults", g.events)
	}
	adult.Role = model.RoleChild
	if err := h.store.UpdateUser(ctx, adult); err != nil {
		t.Fatal(err)
	}
	if _, err := h.SyncChore(ctx, chore.ID); err != nil {
		t.Fatal(err)
	}
	if len(g.events) != 1 {
		t.Fatal("former adult's entry survived", g.events)
	}
	for key := range g.events {
		if !strings.HasPrefix(key, "Bearer Alex:") {
			t.Fatal("wrong shared calendar retained")
		}
	}
	if err := h.store.DeleteChore(ctx, chore.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := h.SyncChore(ctx, chore.ID); err != nil {
		t.Fatal(err)
	}
	if len(g.events) != 0 {
		t.Fatal("deleted chore left events behind")
	}
}

func TestCalendarProfileDeletionKeepsCredentialsUntilCleanupSucceeds(t *testing.T) {
	h, creator := calendarFixture(t)
	g := calendarFake(t, h)
	ctx := context.Background()
	a := calendarPerson(t, h, "Alice", model.RoleChild, true)
	chore, _ := calendarChoreFor(t, h, creator, a)
	enableCalendarFor(t, h, creator, chore.ID, 200)
	g.failDelete = true
	if err := h.DeleteProfile(ctx, a.ID); err == nil {
		t.Fatal("profile deletion ignored failed calendar cleanup")
	}
	if c, _ := h.store.CalendarConnection(ctx, a.ID); c == nil {
		t.Fatal("lost credentials needed for deletion retry")
	}
	if u, _ := h.store.GetUser(ctx, a.ID); u == nil {
		t.Fatal("profile removed before remote cleanup")
	}
	g.failDelete = false
	// The existing profile API requires assignments to be removed first.
	for _, schedule := range mustCalendarSchedules(t, h, chore.ID) {
		if err := h.store.DeleteSchedule(ctx, schedule.ID); err != nil {
			t.Fatal(err)
		}
	}
	if err := h.DeleteProfile(ctx, a.ID); err != nil {
		t.Fatal(err)
	}
	if len(g.events) != 0 {
		t.Fatal("profile deletion left events behind")
	}
	if u, _ := h.store.GetUser(ctx, a.ID); u != nil {
		t.Fatal("profile not deleted")
	}
}

func mustCalendarSchedules(t *testing.T, h *GoogleCalendarHandler, id int64) []model.ChoreSchedule {
	t.Helper()
	s, err := h.store.ListSchedulesForChore(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	return s
}
