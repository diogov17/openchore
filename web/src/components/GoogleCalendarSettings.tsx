import { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { api, type GoogleCalendarStatus } from '../api';
import ui from './admin/ui.module.css';

export function GoogleCalendarSettings() {
  const { t } = useTranslation();
  const [status, setStatus] = useState<GoogleCalendarStatus | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState(() => new URLSearchParams(window.location.search).get('calendar') === 'failed' ? t('calendar.connectFailed') : '');
  useEffect(() => { let live = true; api.googleCalendar.status().then(s => { if (live) setStatus(s); }).catch(() => { if (live) setError(t('calendar.loadError')); }); return () => { live = false; }; }, [t]);
  const action = async (run: () => Promise<unknown>) => {
    setBusy(true); setError('');
    try { await run(); setStatus(await api.googleCalendar.status()); }
    catch (e) { setError(e instanceof Error ? e.message : t('calendar.loadError')); }
    finally { setBusy(false); }
  };
  return <div className={ui.page}>
    <p className={ui.help}>{t('calendar.settingsHelp')}</p>
    {error && <p role="alert" className={ui.msgError}>{error}</p>}
    {!status && !error && <p role="status">{t('chores.loading')}</p>}
    {status && <>
      {!status.configured && <p>{t('calendar.notConfigured')}</p>}
      {status.error && <p role="alert">{t('calendar.reconnectHelp')}</p>}
      {status.connected && <>
        <p>{t('calendar.connected')}</p>
        <label className={ui.field}>
          <span className={ui.label}>{t('calendar.defaultCalendar')}</span>
          <select className={ui.input} value={status.calendar_id === 'primary' ? status.calendars.find(c => c.primary)?.id ?? 'primary' : status.calendar_id} disabled={busy || !!status.error} onChange={e => void action(() => api.googleCalendar.select(e.target.value))}>
            {!status.calendars.length && <option value={status.calendar_id}>{status.calendar_id}</option>}
            {status.calendars.map(c => <option key={c.id} value={c.id}>{c.summary} ({c.id})</option>)}
          </select>
        </label>
      </>}
      <div className={ui.actionsEnd}>
        {status.connected && <button type="button" className={ui.btn} disabled={busy} onClick={() => void action(() => api.googleCalendar.disconnect())}>{t('calendar.disconnect')}</button>}
        {status.configured && <button type="button" className={ui.btnPrimary} disabled={busy} onClick={() => void action(async () => { const { url } = await api.googleCalendar.connect(); window.location.assign(url); })}>{t(status.connected ? 'calendar.reconnect' : 'calendar.connect')}</button>}
      </div>
    </>}
  </div>;
}

export function CalendarCreationToggle({ enabled, onChange }: { enabled: boolean; onChange: (value: boolean) => void }) {
  const { t } = useTranslation();
  const [status, setStatus] = useState<GoogleCalendarStatus | null>(null);
  const [error, setError] = useState(false);
  useEffect(() => { let live = true; api.googleCalendar.status().then(s => { if (live) setStatus(s); }).catch(() => { if (live) setError(true); }); return () => { live = false; }; }, []);
  const ready = !!status?.configured;
  return <div className={ui.field}>
    <label className={ui.check}><input type="checkbox" checked={enabled} disabled={!ready} onChange={e => onChange(e.target.checked)} />{t('calendar.add')}</label>
    <span className={ui.help}>{error ? t('calendar.loadError') : !status ? t('chores.loading') : !ready ? t('calendar.notConfigured') : t('calendar.assigneesHelp')}</span>
    {enabled && <span className={ui.help}>{t('calendar.creationHelp')}</span>}
  </div>;
}
