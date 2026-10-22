import { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { api } from '../api';
import ui from './admin/ui.module.css';

export function ChoreHistory({ choreId }: { choreId: number }) {
  const { t, i18n } = useTranslation();
  const [entries, setEntries] = useState<Awaited<ReturnType<typeof api.chores.history>> | null>(null);
  const [error, setError] = useState(false);
  useEffect(() => {
    let cancelled = false;
    setEntries(null); setError(false);
    api.chores.history(choreId).then(data => { if (!cancelled) setEntries(data); }).catch(() => { if (!cancelled) setError(true); });
    return () => { cancelled = true; };
  }, [choreId]);
  return <section className={ui.section}>
    <h3 className={ui.sectionTitle}>{t('chores.history')}</h3>
    {error ? <p role="alert">{t('chores.historyError')}</p> : entries === null ? <p role="status">{t('chores.loading')}</p> : entries.length === 0 ? <p>{t('chores.noHistory')}</p> :
      <ol style={{ margin: 0, paddingInlineStart: 24 }}>
        {entries.map((e, i) => <li key={`${e.at}-${e.event}-${i}`} style={{ paddingBlock: 8 }}>
          <strong>{t(`chores.events.${e.event}`, { defaultValue: e.event })}</strong>
          {e.person && <> · {e.person}</>}
          {e.points !== 0 && <> · {t('design.chore.points', { count: e.points })}</>}
          <div className={ui.sectionDesc}><time dateTime={e.at}>{new Intl.DateTimeFormat(i18n.language, { dateStyle: 'medium', ...(['available', 'missed'].includes(e.event) ? { timeZone: 'UTC' } : { timeStyle: 'short' as const }) }).format(new Date(e.at))}</time></div>
          {e.date && e.at.slice(0, 10) !== e.date && <div className={ui.sectionDesc}>{t('chores.recordedFor', { date: new Intl.DateTimeFormat(i18n.language, { dateStyle: 'medium' }).format(new Date(`${e.date}T12:00:00`)) })}</div>}
        </li>)}
      </ol>}
  </section>;
}
