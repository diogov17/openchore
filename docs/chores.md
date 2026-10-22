# Adding and completing chores

On Today, use **+**, enter a chore name, and choose **Save** or **Save and add
another**. A name is enough: the chore belongs to the signed-in person, is
independent of daily prerequisites, has no points or proof requirements, and
stays available until completed.

**More options** adds a description, start date, and repetition. Adults can
also choose an assignee, points, a reward rule, approval, and photo proof.
Children can create independent zero-point chores for themselves.

## Unfinished chores

- **Until completed** is the default, including for existing chores after
  migration 028. An unfinished chore remains in Today. Repeating schedules keep
  one open occurrence per chore and assignee; completing it skips intervening
  repetitions and the next scheduled date starts the next occurrence.
- **Start fresh each scheduled day** keeps occurrences tied to their scheduled
  dates. Unfinished past occurrences appear as missed in chore history. Daily
  missed-chore penalties and points decay apply to these resettable obligations.

The same choice is available in the creation wizard and chore editor. YAML
configuration uses `reset_daily: true` to opt into resetting; omission means
until completed.

Completed chores move into the collapsed **Done** section. Undo is available
immediately after completion and from the completed chore's check. Pending
approval remains pending across midnight instead of becoming another chore.

## Chore history

Tap a chore name to see its history. Adults can also edit the chore and its
repetition there. History includes creation, completion/approval status,
credited person and points, undo, and Home backlog repeat rounds. Availability
and missed occurrences are calculated from the saved schedule; editing that
schedule changes those calculated entries. Completion records are preserved
when a chore repeats or its completion is undone.

Week previews show chores on their scheduled dates, with carried chores in
today's column and actual completions on the dates they were completed.
Earlier days in Week can also be checked off when you forgot to record a chore.
The completion belongs to the day selected in Week, rather than today. Future
days remain read-only, and daily prerequisites use the selected day's chores.
