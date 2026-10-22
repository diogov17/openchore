package api

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/liftedkilt/openchore/internal/model"
	"github.com/liftedkilt/openchore/internal/store"
)

type PushHandler struct {
	store *store.Store
	publicKey string
}

func NewPushHandler(s *store.Store) *PushHandler {
	return &PushHandler{store: s, publicKey: os.Getenv("PUSH_VAPID_PUBLIC_KEY")}
}

func (h *PushHandler) PublicKey(w http.ResponseWriter, r *http.Request) {
	if h.publicKey == "" { writeError(w, http.StatusServiceUnavailable, "push notifications are not configured"); return }
	writeJSON(w, http.StatusOK, map[string]string{"public_key": h.publicKey})
}

func (h *PushHandler) Subscribe(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())
	if user == nil || user.ID == 0 { writeError(w, http.StatusUnauthorized, "sign in with a profile to enable reminders"); return }
	var req struct {
		Endpoint string `json:"endpoint"`
		Keys struct { P256dh string `json:"p256dh"`; Auth string `json:"auth"` } `json:"keys"`
		Timezone string `json:"timezone"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Endpoint == "" || req.Keys.P256dh == "" || req.Keys.Auth == "" {
		writeError(w, http.StatusBadRequest, "invalid push subscription"); return
	}
	if !strings.HasPrefix(req.Endpoint, "https://") || len(req.Endpoint) > 4096 { writeError(w, http.StatusBadRequest, "invalid push endpoint"); return }
	if req.Timezone == "" { req.Timezone = "UTC" }
	if _, err := time.LoadLocation(req.Timezone); err != nil { writeError(w, http.StatusBadRequest, "invalid timezone"); return }
	if err := h.store.SavePushSubscription(r.Context(), &model.PushSubscription{UserID:user.ID, Endpoint:req.Endpoint, P256dh:req.Keys.P256dh, Auth:req.Keys.Auth, Timezone:req.Timezone}); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save push subscription"); return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *PushHandler) Status(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())
	if user == nil || user.ID == 0 { writeError(w, http.StatusUnauthorized, "sign in with a profile to manage reminders"); return }
	var req struct { Endpoint string `json:"endpoint"` }
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil { writeError(w, http.StatusBadRequest, "invalid request body"); return }
	exists := false
	if req.Endpoint != "" {
		var err error
		exists, err = h.store.HasPushSubscription(r.Context(), user.ID, req.Endpoint)
		if err != nil { writeError(w, http.StatusInternalServerError, "failed to check push subscription"); return }
	}
	deviceCount, err := h.store.CountPushSubscriptions(r.Context(), user.ID)
	if err != nil { writeError(w, http.StatusInternalServerError, "failed to count push subscriptions"); return }
	writeJSON(w, http.StatusOK, map[string]any{"enabled": exists, "device_count": deviceCount})
}

func (h *PushHandler) Unsubscribe(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())
	if user == nil || user.ID == 0 { writeError(w, http.StatusUnauthorized, "sign in with a profile to manage reminders"); return }
	var req struct { Endpoint string `json:"endpoint"` }
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Endpoint == "" { writeError(w, http.StatusBadRequest, "endpoint is required"); return }
	if err := h.store.DeletePushSubscription(r.Context(), user.ID, req.Endpoint); err != nil { writeError(w, http.StatusInternalServerError, "failed to remove push subscription"); return }
	w.WriteHeader(http.StatusNoContent)
}
