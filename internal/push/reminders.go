package push

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"time"

	webpush "github.com/SherClockHolmes/webpush-go"
	"github.com/liftedkilt/openchore/internal/model"
	"github.com/liftedkilt/openchore/internal/store"
)

type ReminderChecker struct {
	store *store.Store
	publicKey, privateKey, subject string
}

func NewReminderChecker(s *store.Store) *ReminderChecker {
	subject := os.Getenv("PUSH_VAPID_SUBJECT")
	if subject == "" { subject = "admin@openchore.local" }
	return &ReminderChecker{store:s, publicKey:os.Getenv("PUSH_VAPID_PUBLIC_KEY"), privateKey:os.Getenv("PUSH_VAPID_PRIVATE_KEY"), subject:subject}
}

func (c *ReminderChecker) Start(ctx context.Context) {
	if c.publicKey == "" || c.privateKey == "" { log.Printf("push reminders disabled: set PUSH_VAPID_PUBLIC_KEY and PUSH_VAPID_PRIVATE_KEY"); return }
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for {
		c.check(ctx)
		select { case <-ctx.Done(): return; case <-ticker.C: }
	}
}

func (c *ReminderChecker) check(ctx context.Context) {
	subs, err := c.store.ListPushSubscriptions(ctx)
	if err != nil { log.Printf("push reminders: list subscriptions: %v", err); return }
	for _, sub := range subs {
		loc, err := time.LoadLocation(sub.Timezone)
		if err != nil { log.Printf("push reminders: invalid timezone %q: %v", sub.Timezone, err); continue }
		now := time.Now().In(loc)
		date := now.Format(model.DateFormat)
		chores, err := c.store.GetScheduledChoresForUser(ctx, sub.UserID, []string{date}, now)
		if err != nil { log.Printf("push reminders: get chores for user %d: %v", sub.UserID, err); continue }
		for _, chore := range chores {
			if chore.Completed || chore.ReminderAt == nil || *chore.ReminderAt == "" || now.Format("15:04") < *chore.ReminderAt { continue }
			sent, err := c.store.PushReminderSent(ctx, sub.ID, chore.ScheduleID, date)
			if err != nil || sent { continue }
			payload, _ := json.Marshal(map[string]string{"title":"OpenChore reminder", "body":chore.Title, "url":"/"})
			webSub := &webpush.Subscription{Endpoint:sub.Endpoint, Keys:webpush.Keys{P256dh:sub.P256dh, Auth:sub.Auth}}
			resp, err := webpush.SendNotification(payload, webSub, &webpush.Options{Subscriber:c.subject, VAPIDPublicKey:c.publicKey, VAPIDPrivateKey:c.privateKey, TTL:3600, Urgency:webpush.UrgencyHigh})
			if err != nil { log.Printf("push reminders: send to subscription %d: %v", sub.ID, err); continue }
			resp.Body.Close()
			if resp.StatusCode == 404 || resp.StatusCode == 410 { _ = c.store.DeletePushSubscriptionByID(ctx, sub.ID); continue }
			if resp.StatusCode < 200 || resp.StatusCode >= 300 { log.Printf("push reminders: push service returned %s", resp.Status); continue }
			if err := c.store.RecordPushReminder(ctx, sub.ID, chore.ScheduleID, date); err != nil { log.Printf("push reminders: record delivery: %v", err) }
			log.Printf("push reminders: sent schedule %d to user %d subscription %d", chore.ScheduleID, sub.UserID, sub.ID)
		}
	}
}
