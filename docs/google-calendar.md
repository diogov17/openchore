# Google Calendar

Each person connects their own Google account under **Me → Settings → Google Calendar** and chooses a default writable calendar. This is separate from Google sign-in. Chores created with **Add to Google Calendar** enabled in the Schedule step are added to **each assignee's** chosen calendar. The creator's account is used only if the creator is also assigned. Shared household schedules use all adult profiles' calendars, excluding children.

## One-time server setup

1. Create a project in [Google Cloud Console](https://console.cloud.google.com/), enable **Google Calendar API**, and configure the Google Auth Platform consent screen. For a private household app, add the household Google accounts as test users while testing.
2. Create an OAuth client of type **Web application**. Add this exact authorized redirect URI, replacing the hostname with the HTTPS address you use to access OpenChore:

   ```text
   https://your-private-hostname/api/google-calendar/callback
   ```

3. Set these values in the server's ignored `.env` file:

   ```dotenv
   GOOGLE_CALENDAR_CLIENT_ID=your-client-id.apps.googleusercontent.com
   GOOGLE_CALENDAR_CLIENT_SECRET=your-client-secret
   GOOGLE_CALENDAR_REDIRECT_URL=https://your-private-hostname/api/google-calendar/callback
   ```

4. Rebuild and restart the application services:

   ```sh
   docker compose up -d --build api web
   ```

5. Open **Me → Settings → Google Calendar**, select **Connect Google account**, and allow calendar-event access and read-only access to your calendar list. Choose the default calendar after returning to OpenChore.

Google's redirect rules require HTTPS and an eligible hostname, with a localhost exception for development. Raw LAN IP addresses and `.home` names are not eligible. Use a Google-accepted hostname that resolves to your private server. The browser follows the callback; the server does **not** need public inbound access or port forwarding. It needs outbound HTTPS access to Google. Devices must trust your HTTPS certificate.

Google OAuth apps in **Testing** normally receive refresh tokens that expire after seven days when Calendar scopes are requested. For continued household use, configure the consent screen's publishing status appropriately; Google may show an unverified-app screen or require verification depending on distribution. See [Google's OAuth documentation](https://developers.google.com/identity/protocols/oauth2/web-server).

## Event behavior

- The checkbox starts off. Turning it on enables ongoing assignment and schedule sync; a dated schedule and an assignment are required. The creator does not need a Google connection to enable it for other people. Unscheduled Home backlog chores cannot be exported until scheduled.
- A one-off date creates one event per assignee. Weekly schedules create weekly events; interval schedules repeat every N days. Matching duplicate schedules for the same person are combined. Each person receives their own entry, even if several people select the same shared Google calendar.
- The event uses the chore title and description. The available-at time becomes its start, and estimated minutes becomes its duration (30 minutes if unspecified). Without a start time, it is an all-day event. Times use the creator's browser timezone, including daylight saving changes.
- Events are marked private. The chosen calendar's sharing policies still apply.
- Removing an assignment deletes that person's event. Reassigning a schedule removes the old person's event and creates one for the new person. Adding schedules to a calendar-enabled chore creates entries automatically. Title, description, schedule, and default-calendar changes are reconciled too. Deleting a chore removes all its tracked entries. Removing someone from the adult group removes their shared-chore entries on the next retry cycle.
- Completion and missed/reset occurrences are not mirrored into Google Calendar. A recurring event continues while its schedule remains assigned. Changes made in Google Calendar are not imported into OpenChore.
- If a person has not connected Google, or Google is unavailable, the chore and its calendar opt-in remain saved. You can retry immediately or finish and let the server retry automatically. A background reconciliation runs at startup and every minute, so outstanding additions/removals survive restarts and can complete after a person reconnects.
- Event IDs and their original calendar are saved before contacting Google. A lost response can be retried without creating duplicates; removal uses the original calendar even if the person has changed their default. The wizard also keeps successful schedule creations when retrying a failed schedule save.
- Disconnect removes the local tokens; existing events remain in Google Calendar. To withdraw the permission grant itself, also remove OpenChore from your [Google account connections](https://myaccount.google.com/connections).

## Credentials and privacy

The server requests `calendar.events` and `calendar.calendarlist.readonly`. It does not request Gmail or contact access. Tokens are encrypted using AES-GCM, bound to the person's profile, in a separate database table and are never returned to the browser. The encryption key is derived from the server's session secret. Back up that secret with the database; changing it requires reconnecting Calendar accounts. If the session secret is stored in the database, protect the database and backups as credentials, since encryption does not protect against someone who has both the ciphertext and its key.

Only authenticated profile sessions can manage Calendar connections or enable sync. An API token or photo-upload link cannot enable sync. Adults can enable calendar sync for assigned household members; children can enable only their own independently created chores. Assignment changes to an enabled chore trigger server-side sync using the assignees' existing grants. Each person can disconnect only their own account. Deleting a profile first removes tracked Google events, then deletes its stored connection; if cleanup fails, profile deletion is blocked so its credentials remain available for a retry.

Reference: [Google Calendar event insertion](https://developers.google.com/workspace/calendar/api/v3/reference/events/insert), [Calendar list](https://developers.google.com/workspace/calendar/api/v3/reference/calendarList/list).
