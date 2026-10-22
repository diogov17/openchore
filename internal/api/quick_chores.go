package api

import (
	"net/http"
	"strings"
	"time"

	"github.com/liftedkilt/openchore/internal/model"
)

// Quick chores are created atomically, with their initial assignment. A child
// may add a zero-point independent chore for themselves, never award points
// or assign work to another profile.
func (h *ChoreHandler) QuickChore(w http.ResponseWriter, r *http.Request) {
	caller := UserFromContext(r.Context())
	if caller == nil {
		writeError(w, 401, "sign in to add a chore")
		return
	}
	var req struct {
		Title            string `json:"title"`
		Description      string `json:"description"`
		AssignedTo       int64  `json:"assigned_to"`
		Category         string `json:"category"`
		Points           int    `json:"points_value"`
		ResetDaily       bool   `json:"reset_daily"`
		Date             string `json:"date"`
		Interval         int    `json:"interval"`
		RequiresApproval bool   `json:"requires_approval"`
		RequiresPhoto    bool   `json:"requires_photo"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, 400, "invalid request body")
		return
	}
	req.Title = strings.TrimSpace(req.Title)
	if req.Title == "" || len(req.Title) > 500 {
		writeError(w, 400, "enter a chore name (up to 500 characters)")
		return
	}
	if req.AssignedTo == 0 {
		req.AssignedTo = caller.ID
	}
	if req.Category == "" {
		req.Category = model.CategoryIndependent
	}
	if req.Category != model.CategoryIndependent && req.Category != model.CategoryRequired && req.Category != model.CategoryCore && req.Category != model.CategoryBonus && req.Category != model.CategoryHome {
		writeError(w, 400, "invalid chore category")
		return
	}
	if caller.Role != model.RoleAdmin && (req.AssignedTo != caller.ID || req.Points != 0 || req.Category != model.CategoryIndependent || req.RequiresApproval || req.RequiresPhoto) {
		writeError(w, 403, "you can add independent zero-point chores for yourself")
		return
	}
	if req.Points < 0 || req.Interval < 0 || req.Interval > 3650 {
		writeError(w, 400, "invalid points or repeat interval")
		return
	}
	assignee, err := h.store.GetUser(r.Context(), req.AssignedTo)
	if err != nil || assignee == nil {
		writeError(w, 400, "person not found")
		return
	}
	if req.Date == "" {
		req.Date = time.Now().Format(model.DateFormat)
	}
	if _, err := time.Parse(model.DateFormat, req.Date); err != nil {
		writeError(w, 400, "invalid start date")
		return
	}
	chore := &model.Chore{Title: req.Title, Description: strings.TrimSpace(req.Description), Category: req.Category, PointsValue: req.Points, ResetDaily: req.ResetDaily, RequiresApproval: req.RequiresApproval, RequiresPhoto: req.RequiresPhoto, PhotoSource: model.PhotoSourceChild, Source: "manual", CreatedBy: caller.ID}
	if err := h.store.CreateQuickChore(r.Context(), chore, req.AssignedTo, req.Date, req.Interval); err != nil {
		writeError(w, 500, "could not save chore")
		return
	}
	writeJSON(w, http.StatusCreated, chore)
}

func (h *ChoreHandler) ChoreHistory(w http.ResponseWriter, r *http.Request) {
	id, err := urlParamInt64(r, "id")
	if err != nil {
		writeError(w, 400, "invalid chore id")
		return
	}
	caller := UserFromContext(r.Context())
	if caller == nil {
		writeError(w, 401, "sign in to view chore history")
		return
	}
	allowed, err := h.store.CanViewChore(r.Context(), id, caller.ID, caller.Role == model.RoleAdmin)
	if err != nil {
		writeError(w, 500, "could not load chore")
		return
	}
	if !allowed {
		writeError(w, 404, "chore not found")
		return
	}
	viewer := caller.ID
	if caller.Role == model.RoleAdmin {
		viewer = 0
	}
	history, err := h.store.ChoreHistory(r.Context(), id, viewer, time.Now())
	if err != nil {
		writeError(w, 500, "could not load chore history")
		return
	}
	writeJSON(w, 200, history)
}
