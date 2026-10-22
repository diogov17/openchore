# Phone chore reminders

OpenChore sends reminders as Web Push notifications. The API must be reachable
from the phone over HTTPS (except for local development on localhost), and the
browser must be allowed to show notifications. On Android, open the site in
Chrome, sign in to the profile that should receive reminders, then enable
**Chore reminders on this phone** under **Me → Settings**. The existing home
screen shortcut can then receive notifications while OpenChore is closed.

Generate one VAPID key pair with Node.js:

```sh
node scripts/generate-vapid-keys.mjs
```

Copy the three printed values to the `.env` file beside `compose.yaml`. Keep
`PUSH_VAPID_PRIVATE_KEY` private, and keep the same public/private pair when
recreating or updating the container so existing device subscriptions continue
to work. The script also prints `PUSH_VAPID_SUBJECT`; Compose supplies a
default if it is omitted. Then rebuild/restart the Compose stack. The API
checks reminders every 10 seconds and sends one notification per matching
chore, device, and local calendar day, only while the chore remains incomplete.
Changing a chore's reminder time allows it to be sent again that day if the new
time has already arrived.

Set a reminder in **Manage → Chores → Schedules** by choosing a time in the
**Phone reminder** field. Each device must have notifications enabled for the
profile that should receive reminders. The device's timezone is saved when
reminders are enabled there, so the reminder time follows that device's local
time. Push delivery timing can still depend on the phone's operating system
and push service.
