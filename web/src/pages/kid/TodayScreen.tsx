import React from 'react';
import { useTranslation } from 'react-i18next';
import { DayProgress, Greeting, catFromCategory, salutationFor, type DayProgressItem } from '../../design';
import type { HomeBacklogItem, ScheduledChore, User } from '../../types';
import { formatDuration, gates, isDone, type ChoreView } from './choreView';
import { GoalNudge } from './GoalCard';
import { CAT_ORDER, ChoreGroups, Notice, PausedNotice } from './parts';
import type { KidData } from './useKidData';
import { localDateStr } from '../../utils';
import s from './kid.module.css';

/** The day runs 7 am – 9 pm on the sun arc. */
const DAY_FROM = 7;
const DAY_TO = 21;
export const dayFraction = (d: Date) =>
  Math.min(1, Math.max(0, (d.getHours() + d.getMinutes() / 60 - DAY_FROM) / (DAY_TO - DAY_FROM)));

interface TodayProps {
  user: User;
  data: KidData;
  now: Date;
  topBar: React.ReactNode;
  viewOf: (c: ScheduledChore) => ChoreView;
  tts: boolean;
  onToggle: (c: ScheduledChore, view: ChoreView) => void;
  onPhoto: (c: ScheduledChore) => void;
  onSpeak: (text: string, audioUrl?: string) => void;
}

export function TodayScreen({ user, data, now, topBar, viewOf, tts, onToggle, onPhoto, onSpeak }: TodayProps) {
  const { t, i18n } = useTranslation();
  const scheduledToday = (data.chores ?? []).filter(c => c.date === data.today);
  const completedToday = (item: HomeBacklogItem) => !!item.completed_at && (
    item.completion_date === data.today || localDateStr(new Date(item.completed_at)) === data.today
  );
  // Assigned Home chores persist until completed. Keep those open rows on
  // Today and show any Home completion from today under the same category.
  const homeItems = new Map<number, HomeBacklogItem>();
  data.homeBacklog.filter(completedToday).forEach(item => homeItems.set(item.chore_id, item));
  data.assignedHome
    .filter(item => !item.completed_at || completedToday(item))
    .forEach(item => {
      const existing = homeItems.get(item.chore_id);
      // A late open-assignment response must not overwrite a completion just
      // loaded from the shared Home backlog.
      if (!existing || item.completed_at) homeItems.set(item.chore_id, item);
    });
  const homeToday = [...homeItems.values()]
    .map(item => ({
      schedule_id: -item.chore_id,
      chore_id: item.chore_id,
      title: item.title,
      description: item.description,
      category: 'home' as const,
      icon: item.icon,
      points_value: item.completed_at ? item.points_awarded ?? item.points_value : item.points_value,
      estimated_minutes: item.estimated_minutes,
      missed_penalty_value: 0,
      requires_approval: false,
      requires_photo: false,
      assignment_type: 'family',
      expiry_penalty: 'block' as const,
      expiry_penalty_value: 0,
      available: true,
      expired: false,
      completed: !!item.completed_at,
      completed_at: item.completed_at ?? undefined,
      date: data.today,
    }));
  const chores = [...scheduledToday, ...homeToday];
  const todayViewOf = (chore: ScheduledChore): ChoreView => {
    const view = viewOf(chore);
    if (chore.schedule_id >= 0) return view;
    const item = homeItems.get(chore.chore_id);
    if (!chore.completed) {
      return { ...view, cat: 'daily', state: 'todo', canToggle: true, points: chore.points_value };
    }
    const date = chore.completed_at ? new Date(chore.completed_at) : null;
    const duration = chore.estimated_minutes
      ? t('kid.chore.about', { time: formatDuration(chore.estimated_minutes, t) })
      : '';
    return {
      ...view,
      cat: 'daily',
      state: 'done',
      canToggle: !!item,
      points: chore.points_value,
      meta: [duration, t('kid.home.completedBy', {
        name: item?.completed_by_name || t('kid.home.unknown'),
        date: date && !Number.isNaN(date.getTime())
          ? new Intl.DateTimeFormat(i18n.language, { dateStyle: 'medium', timeStyle: 'short' }).format(date)
          : '',
      })].filter(Boolean).join(' · '),
    };
  };
  const firstName = user.name.split(' ')[0];

  // One mark per chore, in list order, so the shape row reads like the list.
  const items: DayProgressItem[] = CAT_ORDER.flatMap(cat =>
    chores.filter(c => catFromCategory(c.category) === cat).map(c => {
      const done = isDone(c);
      const at = done && c.completed_at ? new Date(c.completed_at) : null;
      return { cat, done, at: at && !Number.isNaN(at.getTime()) ? dayFraction(at) : undefined };
    }));
  const done = items.filter(i => i.done).length;
  const allDone = items.length > 0 && done === items.length;

  // Points today, as the old stats showed them: Every day points wait until
  // every Must do is done.
  const { requiredDone } = gates(chores);
  let earned = 0;
  let pending = 0;
  for (const c of chores) {
    if (!c.completed) continue;
    if (c.category === 'core' && !requiredDone) pending += c.points_value || 0;
    else earned += c.points_value || 0;
  }

  const commitments = data.points?.active_commitments ?? [];
  const loading = data.chores == null;

  return (
    <div className={s.today}>
      <section className={s.todayHero} aria-label={t('kid.today.overview')}>
        {topBar}
        <Greeting salutation={t(`design.greeting.${salutationFor(now)}`)} name={firstName} />
        {!user.paused && items.length > 0 && (
          <div role="group" aria-label={t('design.progress.label', { done, total: items.length })} className={s.progress}>
            <DayProgress items={items} now={dayFraction(now)} />
          </div>
        )}
        {!user.paused && items.length > 0 && (
          <p className={s.todayLine}>
            {allDone ? <strong>{t('kid.today.allDone')}</strong> : null}
            {earned > 0 && <span>{t('kid.today.earned', { count: earned })}</span>}
            {pending > 0 && <span>{t('kid.today.pending', { count: pending })}</span>}
          </p>
        )}
      </section>

      <section className={s.todayLists} aria-label={t('kid.today.listLabel')}>
        {user.paused ? (
          <PausedNotice />
        ) : loading ? null : chores.length === 0 ? (
          <Notice icon="sprout" title={t('kid.today.emptyTitle')}>{t('kid.today.emptyText')}</Notice>
        ) : (
          <ChoreGroups
            chores={chores}
            viewOf={todayViewOf}
            togglingIds={data.togglingIds}
            tts={tts}
            onToggle={onToggle}
            onPhoto={onPhoto}
            onSpeak={onSpeak}
          />
        )}
      </section>

      {/* Goals: under the list on a phone, under the hero on a tablet. */}
      {commitments.length > 0 && (
        <section className={s.nudges} aria-label={t('kid.goal.title')}>
          {commitments.map(c => <GoalNudge key={c.id} c={c} me={user} people={data.people} />)}
        </section>
      )}
    </div>
  );
}
