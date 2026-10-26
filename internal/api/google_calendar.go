package api

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/liftedkilt/openchore/internal/model"
	"github.com/liftedkilt/openchore/internal/store"
	"golang.org/x/oauth2"
)

const calendarScope = "https://www.googleapis.com/auth/calendar.events"
const calendarListScope = "https://www.googleapis.com/auth/calendar.calendarlist.readonly"
const calendarCookie = "openchore_calendar_flow"

type calendarFlow struct {
	userID, version int64
	state, verifier string
	expires         time.Time
}

type GoogleCalendarHandler struct {
	store  *store.Store
	config oauth2.Config
	seal   cipher.AEAD
	client *http.Client
	apiURL string
	mu     sync.Mutex // serializes credential refresh, disconnect and event insertion
	flows  map[string]calendarFlow
	flowMu sync.Mutex
}

func NewGoogleCalendarHandler(s *store.Store, sm *SessionManager) *GoogleCalendarHandler {
	key := sha256.Sum256(append([]byte("openchore/google-calendar/v1/"), sm.secret...))
	block, _ := aes.NewCipher(key[:])
	seal, _ := cipher.NewGCM(block)
	return &GoogleCalendarHandler{store: s, seal: seal, client: &http.Client{Timeout: 20 * time.Second}, apiURL: "https://www.googleapis.com/calendar/v3", flows: map[string]calendarFlow{}, config: oauth2.Config{
		ClientID: os.Getenv("GOOGLE_CALENDAR_CLIENT_ID"), ClientSecret: os.Getenv("GOOGLE_CALENDAR_CLIENT_SECRET"), RedirectURL: os.Getenv("GOOGLE_CALENDAR_REDIRECT_URL"),
		Endpoint: oauth2.Endpoint{AuthURL: "https://accounts.google.com/o/oauth2/v2/auth", TokenURL: "https://oauth2.googleapis.com/token"}, Scopes: []string{calendarScope, calendarListScope},
	}}
}

func (h *GoogleCalendarHandler) configured() bool {
	return h.config.ClientID != "" && h.config.ClientSecret != "" && h.config.RedirectURL != ""
}
func (h *GoogleCalendarHandler) save(ctx context.Context, userID int64, token *oauth2.Token, calendar string) error {
	raw, err := json.Marshal(token)
	if err != nil {
		return err
	}
	nonce := make([]byte, h.seal.NonceSize())
	if _, err = rand.Read(nonce); err != nil {
		return err
	}
	aad := []byte(strconv.FormatInt(userID, 10))
	return h.store.SaveCalendarConnection(ctx, &store.CalendarConnection{UserID: userID, CalendarID: calendar, Token: h.seal.Seal(nonce, nonce, raw, aad)})
}
func (h *GoogleCalendarHandler) decrypt(c *store.CalendarConnection) (*oauth2.Token, error) {
	n := h.seal.NonceSize()
	if len(c.Token) < n {
		return nil, errors.New("invalid stored credentials")
	}
	raw, err := h.seal.Open(nil, c.Token[:n], c.Token[n:], []byte(strconv.FormatInt(c.UserID, 10)))
	if err != nil {
		return nil, err
	}
	var token oauth2.Token
	err = json.Unmarshal(raw, &token)
	return &token, err
}
func (h *GoogleCalendarHandler) token(ctx context.Context, c *store.CalendarConnection) (*oauth2.Token, error) {
	token, err := h.decrypt(c)
	if err != nil {
		return nil, err
	}
	ctx = context.WithValue(ctx, oauth2.HTTPClient, h.client)
	fresh, err := h.config.TokenSource(ctx, token).Token()
	if err != nil {
		return nil, errors.New("Google Calendar connection expired; reconnect in Me settings")
	}
	if fresh.AccessToken != token.AccessToken || fresh.Expiry != token.Expiry {
		if err = h.save(ctx, c.UserID, fresh, c.CalendarID); err != nil {
			return nil, err
		}
	}
	return fresh, nil
}
func (h *GoogleCalendarHandler) request(ctx context.Context, token *oauth2.Token, method, path string, body any, out any) (int, error) {
	var data []byte
	var err error
	if body != nil {
		data, err = json.Marshal(body)
		if err != nil {
			return 0, err
		}
	}
	req, err := http.NewRequestWithContext(ctx, method, h.apiURL+path, bytes.NewReader(data))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)
	req.Header.Set("Content-Type", "application/json")
	resp, err := h.client.Do(req)
	if err != nil {
		return 0, errors.New("Google Calendar is unavailable; try again")
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return resp.StatusCode, fmt.Errorf("Google Calendar request failed (%d); check your connection and calendar permissions", resp.StatusCode)
	}
	if out != nil {
		err = json.NewDecoder(io.LimitReader(resp.Body, 2<<20)).Decode(out)
	}
	return resp.StatusCode, err
}

type googleCalendar struct {
	ID         string `json:"id"`
	Summary    string `json:"summary"`
	AccessRole string `json:"accessRole"`
	Primary    bool   `json:"primary"`
}

func (h *GoogleCalendarHandler) calendars(ctx context.Context, tok *oauth2.Token) ([]googleCalendar, error) {
	calendars := []googleCalendar{}
	page := ""
	for {
		var response struct {
			Items []googleCalendar `json:"items"`
			Next  string           `json:"nextPageToken"`
		}
		_, err := h.request(ctx, tok, "GET", "/users/me/calendarList?minAccessRole=writer&maxResults=250&pageToken="+url.QueryEscape(page), nil, &response)
		if err != nil {
			return nil, err
		}
		for _, c := range response.Items {
			if c.AccessRole == "owner" || c.AccessRole == "writer" {
				calendars = append(calendars, c)
			}
		}
		page = response.Next
		if page == "" {
			break
		}
	}
	return calendars, nil
}

// These routes use RequireSession, so API tokens and upload links cannot manage personal credentials.
func (h *GoogleCalendarHandler) Status(w http.ResponseWriter, r *http.Request) {
	h.mu.Lock()
	defer h.mu.Unlock()
	c, err := h.store.CalendarConnection(r.Context(), UserFromContext(r.Context()).ID)
	if err != nil {
		writeError(w, 500, "failed to load Calendar connection")
		return
	}
	result := map[string]any{"configured": h.configured(), "connected": c != nil, "calendar_id": "", "calendars": []googleCalendar{}}
	if c != nil {
		result["calendar_id"] = c.CalendarID
		tok, err := h.token(r.Context(), c)
		if err == nil {
			var items []googleCalendar
			items, err = h.calendars(r.Context(), tok)
			if err == nil {
				result["calendars"] = items
			}
		}
		if err != nil {
			result["error"] = "Reconnect Google Calendar to load your calendars."
		}
	}
	writeJSON(w, 200, result)
}
func (h *GoogleCalendarHandler) Connect(w http.ResponseWriter, r *http.Request) {
	if !h.configured() {
		writeError(w, 503, "Google Calendar has not been configured on this server")
		return
	}
	user := UserFromContext(r.Context())
	state := randomString()
	flow := calendarFlow{userID: user.ID, version: user.SessionVersion, state: state, verifier: oauth2.GenerateVerifier(), expires: time.Now().Add(10 * time.Minute)}
	h.flowMu.Lock()
	for key, f := range h.flows {
		if time.Now().After(f.expires) {
			delete(h.flows, key)
		}
	}
	h.flows[state] = flow
	h.flowMu.Unlock()
	http.SetCookie(w, &http.Cookie{Name: calendarCookie, Value: state, Path: "/api/google-calendar/", HttpOnly: true, Secure: isHTTPS(r), SameSite: http.SameSiteLaxMode, MaxAge: 600})
	writeJSON(w, 200, map[string]string{"url": h.config.AuthCodeURL(state, oauth2.AccessTypeOffline, oauth2.SetAuthURLParam("prompt", "consent"), oauth2.S256ChallengeOption(flow.verifier))})
}
func (h *GoogleCalendarHandler) Callback(w http.ResponseWriter, r *http.Request) {
	result := "failed"
	defer func() { http.Redirect(w, r, "/?calendar="+result, http.StatusSeeOther) }()
	state := r.URL.Query().Get("state")
	cookie, err := r.Cookie(calendarCookie)
	if err != nil || cookie.Value != state {
		return
	}
	h.flowMu.Lock()
	flow, ok := h.flows[state]
	delete(h.flows, state)
	h.flowMu.Unlock()
	http.SetCookie(w, &http.Cookie{Name: calendarCookie, Path: "/api/google-calendar/", HttpOnly: true, Secure: isHTTPS(r), SameSite: http.SameSiteLaxMode, MaxAge: -1})
	user := UserFromContext(r.Context())
	if !ok || time.Now().After(flow.expires) || flow.userID != user.ID || flow.version != user.SessionVersion || r.URL.Query().Get("error") != "" {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	ctx = context.WithValue(ctx, oauth2.HTTPClient, h.client)
	tok, err := h.config.Exchange(ctx, r.URL.Query().Get("code"), oauth2.VerifierOption(flow.verifier))
	if err != nil || tok.RefreshToken == "" {
		return
	}
	if scope, ok := tok.Extra("scope").(string); ok {
		granted := strings.Fields(scope)
		for _, required := range h.config.Scopes {
			found := false
			for _, s := range granted {
				found = found || s == required
			}
			if !found {
				return
			}
		}
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if err = h.save(ctx, user.ID, tok, "primary"); err == nil {
		result = "connected"
	}
}
func (h *GoogleCalendarHandler) Select(w http.ResponseWriter, r *http.Request) {
	var req struct {
		CalendarID string `json:"calendar_id"`
	}
	if decodeJSON(r, &req) != nil {
		writeError(w, 400, "invalid calendar")
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	c, err := h.store.CalendarConnection(r.Context(), UserFromContext(r.Context()).ID)
	if err != nil || c == nil {
		writeError(w, 400, "connect Google Calendar first")
		return
	}
	tok, err := h.token(r.Context(), c)
	if err != nil {
		writeError(w, 502, err.Error())
		return
	}
	items, err := h.calendars(r.Context(), tok)
	if err != nil {
		writeError(w, 502, err.Error())
		return
	}
	for _, item := range items {
		if item.ID == req.CalendarID || (req.CalendarID == "primary" && item.Primary) {
			if err = h.save(r.Context(), c.UserID, tok, req.CalendarID); err != nil {
				writeError(w, 500, "failed to save calendar")
				return
			}
			w.WriteHeader(204)
			return
		}
	}
	writeError(w, 400, "choose a writable calendar")
}
func (h *GoogleCalendarHandler) Disconnect(w http.ResponseWriter, r *http.Request) {
	h.mu.Lock()
	defer h.mu.Unlock()
	// Remove our access immediately. Users can also revoke it in Google account settings.
	if err := h.store.DeleteCalendarConnection(r.Context(), UserFromContext(r.Context()).ID); err != nil {
		writeError(w, 500, "failed to disconnect")
		return
	}
	w.WriteHeader(204)
}

func calendarEvent(chore *model.Chore, s model.ChoreSchedule, timezone string, now time.Time) (map[string]any, error) {
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		return nil, errors.New("invalid timezone")
	}
	day := time.Date(now.In(loc).Year(), now.In(loc).Month(), now.In(loc).Day(), 0, 0, 0, 0, loc)
	parse := func(value string) (time.Time, error) {
		// SQLite DATE columns may be scanned as "YYYY-MM-DDT00:00:00Z".
		if len(value) > 10 {
			value = value[:10]
		}
		return time.ParseInLocation("2006-01-02", value, loc)
	}
	recurrence := ""
	if s.SpecificDate != nil {
		day, err = parse(*s.SpecificDate)
	} else if s.RecurrenceInterval != nil && *s.RecurrenceInterval > 0 && s.RecurrenceStart != nil {
		day, err = parse(*s.RecurrenceStart)
		recurrence = fmt.Sprintf("RRULE:FREQ=DAILY;INTERVAL=%d", *s.RecurrenceInterval)
	} else if s.DayOfWeek != nil {
		if s.StartDate != nil {
			day, err = parse(*s.StartDate)
		}
		day = day.AddDate(0, 0, (*s.DayOfWeek-int(day.Weekday())+7)%7)
		recurrence = "RRULE:FREQ=WEEKLY"
	}
	if err != nil {
		return nil, errors.New("invalid schedule date")
	}
	if recurrence != "" && s.EndDate != nil {
		end, e := parse(*s.EndDate)
		if e != nil {
			return nil, e
		}
		recurrence += ";UNTIL=" + end.Format("20060102")
	}
	event := map[string]any{"summary": chore.Title, "description": chore.Description, "visibility": "private"}
	start := map[string]string{"date": day.Format("2006-01-02")}
	end := map[string]string{"date": day.AddDate(0, 0, 1).Format("2006-01-02")}
	if s.AvailableAt != nil && *s.AvailableAt != "" {
		clock, e := time.Parse("15:04", *s.AvailableAt)
		if e != nil {
			return nil, errors.New("invalid schedule time")
		}
		at := time.Date(day.Year(), day.Month(), day.Day(), clock.Hour(), clock.Minute(), 0, 0, loc)
		minutes := 30
		if chore.EstimatedMinutes != nil && *chore.EstimatedMinutes > 0 {
			minutes = *chore.EstimatedMinutes
		}
		start = map[string]string{"dateTime": at.Format(time.RFC3339), "timeZone": timezone}
		end = map[string]string{"dateTime": at.Add(time.Duration(minutes) * time.Minute).Format(time.RFC3339), "timeZone": timezone}
		if recurrence != "" && s.EndDate != nil {
			d, _ := parse(*s.EndDate)
			index := strings.Index(recurrence, ";UNTIL=")
			recurrence = recurrence[:index] + ";UNTIL=" + d.AddDate(0, 0, 1).Add(-time.Second).UTC().Format("20060102T150405Z")
		}
	}
	event["start"] = start
	event["end"] = end
	if recurrence != "" {
		event["recurrence"] = []string{recurrence}
	}
	return event, nil
}

func (h *GoogleCalendarHandler) AddEvents(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ChoreID  int64  `json:"chore_id"`
		Timezone string `json:"timezone"`
	}
	if decodeJSON(r, &req) != nil || req.ChoreID <= 0 {
		writeError(w, 400, "invalid chore")
		return
	}
	chore, err := h.store.GetChore(r.Context(), req.ChoreID)
	if err != nil || chore == nil {
		writeError(w, 404, "chore not found")
		return
	}
	schedules, err := h.store.ListSchedulesForChore(r.Context(), req.ChoreID)
	if err != nil {
		writeError(w, 500, "failed to load schedules")
		return
	}
	user := UserFromContext(r.Context())
	selected := []model.ChoreSchedule{}
	for _, s := range schedules {
		if user.Role == model.RoleAdmin || s.AssignedTo == user.ID {
			selected = append(selected, s)
		}
	}
	if len(selected) == 0 {
		writeError(w, 400, "a dated schedule assigned to you is required")
		return
	}
	// Non-admin profiles can opt in only their own independently created chore.
	// They must not cause writes to other people's connected calendars.
	if user.Role != model.RoleAdmin {
		if chore.CreatedBy != user.ID || len(selected) != len(schedules) {
			writeError(w, 403, "only the creator or an adult can enable calendar sync")
			return
		}
		for _, s := range selected {
			if s.AssignmentType == model.AssignmentFamily {
				writeError(w, 403, "only an adult can enable shared calendar sync")
				return
			}
		}
	}
	for _, s := range selected {
		_, e := calendarEvent(chore, s, req.Timezone, time.Now())
		if e != nil {
			writeError(w, 400, e.Error())
			return
		}
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if err = h.store.EnableChoreCalendar(r.Context(), store.CalendarChore{ChoreID: chore.ID, Timezone: req.Timezone, AnchorDate: time.Now().UTC().Format(time.RFC3339)}); err != nil {
		writeError(w, 500, "failed to enable chore calendar")
		return
	}
	added, err := h.syncChore(r.Context(), chore.ID)
	if err != nil {
		writeError(w, 502, "Calendar sync enabled; updates are pending and will retry automatically. "+err.Error())
		return
	}
	writeJSON(w, 200, map[string]int{"added": added})
}

// SyncChore reconciles durable event references with the current assignments.
// Mapping rows are written BEFORE contacting Google, so a crash or lost response
// cannot leave an untracked event which would survive assignment removal.
func (h *GoogleCalendarHandler) SyncChore(ctx context.Context, id int64) (int, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.syncChore(ctx, id)
}

func (h *GoogleCalendarHandler) syncChore(ctx context.Context, id int64) (int, error) {
	existing, err := h.store.ChoreCalendarEvents(ctx, id)
	if err != nil {
		return 0, err
	}
	config, err := h.store.ChoreCalendar(ctx, id)
	if err != nil {
		return 0, err
	}
	desired := map[string]store.CalendarEvent{}
	problems := []error{}
	connections := map[int64]*store.CalendarConnection{}
	tokens := map[int64]*oauth2.Token{}
	credential := func(userID int64) (*store.CalendarConnection, *oauth2.Token, error) {
		if c, ok := connections[userID]; ok {
			if c == nil {
				return nil, nil, errors.New("an assignee has not connected Google Calendar")
			}
			if tokens[userID] == nil {
				return nil, nil, errors.New("an assignee must reconnect Google Calendar")
			}
			return c, tokens[userID], nil
		}
		c, e := h.store.CalendarConnection(ctx, userID)
		if e != nil {
			return nil, nil, e
		}
		connections[userID] = c
		if c == nil {
			return nil, nil, errors.New("an assignee has not connected Google Calendar")
		}
		tok, e := h.token(ctx, c)
		if e != nil {
			return c, nil, e
		}
		tokens[userID] = tok
		return c, tok, nil
	}
	// Keep disconnected assignees' existing entries until their connection is
	// restored, but still remove them if the assignment itself has gone away.
	assigned := map[string]bool{}
	if config != nil {
		chore, e := h.store.GetChore(ctx, id)
		if e != nil {
			return 0, e
		}
		if chore == nil {
			return 0, errors.New("chore was removed during calendar sync; retrying cleanup")
		}
		schedules, e := h.store.ListSchedulesForChore(ctx, id)
		if e != nil {
			return 0, e
		}
		users, e := h.store.ListUsers(ctx)
		if e != nil {
			return 0, e
		}
		anchor, e := time.Parse(time.RFC3339, config.AnchorDate)
		if e != nil {
			return 0, e
		}
		seen := map[string]bool{}
		for _, s := range schedules {
			people := []int64{s.AssignedTo}
			if s.AssignmentType == model.AssignmentFamily {
				people = nil
				for _, u := range users {
					if u.Role == model.RoleAdmin {
						people = append(people, u.ID)
					}
				}
			}
			for _, userID := range people {
				assigned[fmt.Sprintf("%d:%d", userID, s.ID)] = true
				c, _, e := credential(userID)
				if e != nil {
					problems = append(problems, e)
					continue
				}
				event, e := calendarEvent(chore, s, config.Timezone, anchor)
				if e != nil {
					problems = append(problems, e)
					continue
				}
				// Explicit empty recurrence clears a previous repeating schedule on PATCH.
				if _, ok := event["recurrence"]; !ok {
					event["recurrence"] = []string{}
				}
				body, _ := json.Marshal(event)
				key := fmt.Sprintf("%d:%s", userID, body)
				if seen[key] {
					continue
				}
				seen[key] = true
				eventID := ""
				for _, old := range existing {
					if old.UserID == userID && old.ScheduleID == s.ID && old.CalendarID == c.CalendarID {
						eventID = old.EventID
						break
					}
				}
				if eventID == "" {
					hash := sha256.Sum256([]byte(randomString()))
					eventID = hex.EncodeToString(hash[:])
				}
				desired[eventID] = store.CalendarEvent{EventID: eventID, ChoreID: id, ScheduleID: s.ID, UserID: userID, CalendarID: c.CalendarID, Payload: string(body)}
			}
		}
	}
	old := map[string]store.CalendarEvent{}
	// Delete obsolete entries first, using the calendar saved at insertion time
	// rather than the person's (possibly changed) default calendar.
	for _, e := range existing {
		old[e.EventID] = e
		if _, ok := desired[e.EventID]; ok {
			continue
		}
		if config != nil && assigned[fmt.Sprintf("%d:%d", e.UserID, e.ScheduleID)] && tokens[e.UserID] == nil {
			continue
		}
		_, tok, err := credential(e.UserID)
		if err != nil {
			problems = append(problems, err)
			continue
		}
		status, err := h.request(ctx, tok, "DELETE", "/calendars/"+url.PathEscape(e.CalendarID)+"/events/"+e.EventID, nil, nil)
		if err != nil && status != 404 && status != 410 {
			problems = append(problems, err)
			continue
		}
		if err = h.store.DeleteCalendarEvent(ctx, e.EventID); err != nil {
			problems = append(problems, err)
		}
	}
	added := 0
	for eventID, e := range desired {
		previous, exists := old[eventID]
		if exists && previous.Payload == e.Payload {
			added++
			continue
		}
		_, tok, err := credential(e.UserID)
		if err != nil {
			problems = append(problems, err)
			continue
		}
		// An empty payload is an outstanding insertion, not an acknowledged event.
		if !exists {
			pending := e
			pending.Payload = ""
			if err = h.store.SaveCalendarEvent(ctx, pending); err != nil {
				problems = append(problems, err)
				continue
			}
		}
		var body map[string]any
		json.Unmarshal([]byte(e.Payload), &body)
		path := "/calendars/" + url.PathEscape(e.CalendarID) + "/events"
		if exists && previous.Payload != "" {
			_, err = h.request(ctx, tok, "PATCH", path+"/"+eventID, body, nil)
		} else {
			body["id"] = eventID
			var status int
			status, err = h.request(ctx, tok, "POST", path, body, nil)
			if status == 409 {
				delete(body, "id")
				_, err = h.request(ctx, tok, "PATCH", path+"/"+eventID, body, nil)
			}
		}
		if err != nil {
			problems = append(problems, err)
			continue
		}
		if err = h.store.SaveCalendarEvent(ctx, e); err != nil {
			problems = append(problems, err)
			continue
		}
		added++
	}
	return added, errors.Join(problems...)
}

func (h *ChoreHandler) syncCalendar(ctx context.Context, id int64) {
	if h.calendar == nil {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	if _, err := h.calendar.SyncChore(ctx, id); err != nil {
		log.Printf("calendar sync pending for chore %d: %v", id, err)
	}
}

// StartCalendarSync retries pending creations/deletions after failures or restarts,
// and picks up new connections and shared-adult membership changes.
func (h *ChoreHandler) StartCalendarSync(ctx context.Context) {
	if h.calendar == nil {
		return
	}
	tick := time.NewTicker(time.Minute)
	defer tick.Stop()
	for {
		ids, err := h.store.CalendarChoreIDs(ctx)
		if err == nil {
			for _, id := range ids {
				h.syncCalendar(ctx, id)
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
	}
}

// Remove remote entries before deleting the profile (and its credentials).
// On failure the profile remains, so retrying can still authenticate to Google.
func (h *GoogleCalendarHandler) DeleteProfile(ctx context.Context, userID int64) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	ids, err := h.store.CalendarChoreIDs(ctx)
	if err != nil {
		return err
	}
	entries := []store.CalendarEvent{}
	for _, id := range ids {
		events, e := h.store.ChoreCalendarEvents(ctx, id)
		if e != nil {
			return e
		}
		for _, event := range events {
			if event.UserID == userID {
				entries = append(entries, event)
			}
		}
	}
	if len(entries) > 0 {
		c, err := h.store.CalendarConnection(ctx, userID)
		if err != nil {
			return err
		}
		if c == nil {
			return errors.New("reconnect Google Calendar before deleting this profile to remove its events")
		}
		tok, err := h.token(ctx, c)
		if err != nil {
			return err
		}
		for _, event := range entries {
			status, e := h.request(ctx, tok, "DELETE", "/calendars/"+url.PathEscape(event.CalendarID)+"/events/"+event.EventID, nil, nil)
			if e != nil && status != 404 && status != 410 {
				return e
			}
			if e = h.store.DeleteCalendarEvent(ctx, event.EventID); e != nil {
				return e
			}
		}
	}
	return h.store.DeleteUser(ctx, userID)
}
