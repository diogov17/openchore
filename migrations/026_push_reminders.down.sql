DROP TABLE IF EXISTS push_reminder_deliveries;
DROP TABLE IF EXISTS push_subscriptions;
ALTER TABLE chore_schedules DROP COLUMN reminder_at;
