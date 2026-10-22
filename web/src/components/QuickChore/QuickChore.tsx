import { useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import Modal from '../Modal/Modal';
import { api } from '../../api';
import type { Chore, User } from '../../types';
import { localDateStr } from '../../utils';
import ui from '../admin/ui.module.css';

export function QuickChore({ user, onClose, onSaved }: { user: User; onClose: () => void; onSaved: () => void }) {
  const { t } = useTranslation();
  const [title, setTitle] = useState('');
  const [description, setDescription] = useState('');
  const [people, setPeople] = useState<User[]>([user]);
  const [assignedTo, setAssignedTo] = useState(user.id);
  const [category, setCategory] = useState<Chore['category']>('independent');
  const [points, setPoints] = useState(0);
  const [date, setDate] = useState(localDateStr(new Date()));
  const [interval, setInterval] = useState(0);
  const [resetDaily, setResetDaily] = useState(false);
  const [approval, setApproval] = useState(false);
  const [photo, setPhoto] = useState(false);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState('');
  const [message, setMessage] = useState('');
  const nameRef = useRef<HTMLInputElement>(null);
  const busy = useRef(false);
  const save = async (another: boolean) => {
    if (!title.trim() || busy.current) return;
    busy.current = true;
    setSaving(true); setError(''); setMessage('');
    try {
      await api.chores.quickAdd({ title: title.trim(), description, assigned_to: assignedTo, category, points_value: points, date, interval, reset_daily: resetDaily, requires_approval: approval, requires_photo: photo });
      onSaved();
      if (another) { setTitle(''); setDescription(''); setMessage(t('chores.saved')); }
      else onClose();
    } catch (e) { setError(e instanceof Error ? e.message : t('kid.errors.couldntSave')); }
    finally { busy.current = false; setSaving(false); requestAnimationFrame(() => nameRef.current?.focus()); }
  };
  return (
    <Modal isOpen title={t('chores.add')} onClose={() => { if (!busy.current) onClose(); }} maxWidth="520px">
      <form className={ui.page} onSubmit={e => { e.preventDefault(); void save(false); }}>
        <label className={ui.field}>
          <span className={ui.label}>{t('chores.name')}</span>
          <input ref={nameRef} autoFocus className={ui.input} value={title} maxLength={500} required disabled={saving} placeholder={t('chores.placeholder')} onChange={e => setTitle(e.target.value)} />
        </label>
        <p className={ui.sectionDesc}>{t('chores.defaults', { name: user.name })}</p>
        <details onToggle={e => { if (e.currentTarget.open && user.role === 'admin') void api.users.list().then(setPeople).catch(() => setError(t('chores.peopleError'))); }}>
          <summary>{t('chores.moreOptions')}</summary>
          <div className={ui.cardStack} style={{ marginTop: 16 }}>
            <label className={ui.field}><span className={ui.label}>{t('chores.description')}</span><textarea className={ui.input} value={description} onChange={e => setDescription(e.target.value)} disabled={saving} /></label>
            {user.role === 'admin' && <>
              <label className={ui.field}><span className={ui.label}>{t('chores.assignee')}</span><select className={ui.input} value={assignedTo} onChange={e => setAssignedTo(Number(e.target.value))} disabled={saving}>{people.map(p => <option key={p.id} value={p.id}>{p.name}</option>)}</select></label>
              <label className={ui.field}><span className={ui.label}>{t('chores.rule')}</span><select className={ui.input} value={category} onChange={e => setCategory(e.target.value as Chore['category'])} disabled={saving}>
                <option value="independent">{t('chores.independent')}</option><option value="required">{t('design.category.essential')}</option><option value="core">{t('design.category.daily')}</option><option value="bonus">{t('design.category.bonus')}</option>
              </select></label>
              <label className={ui.field}><span className={ui.label}>{t('chores.points')}</span><input className={ui.input} type="number" min={0} value={points} onChange={e => setPoints(Number(e.target.value))} disabled={saving} /></label>
            </>}
            <label className={ui.field}><span className={ui.label}>{t('chores.startDate')}</span><input className={ui.input} type="date" required value={date} onChange={e => setDate(e.target.value)} disabled={saving} /></label>
            <label className={ui.field}><span className={ui.label}>{t('chores.repeat')}</span><select className={ui.input} value={interval === 0 ? 'once' : 'repeat'} onChange={e => setInterval(e.target.value === 'once' ? 0 : 1)} disabled={saving}><option value="once">{t('chores.once')}</option><option value="repeat">{t('chores.everyNDays')}</option></select></label>
            {interval > 0 && <label className={ui.field}><span className={ui.label}>{t('chores.interval')}</span><input className={ui.input} type="number" min={1} max={3650} required value={interval} onChange={e => setInterval(Number(e.target.value))} disabled={saving} /></label>}
            <label className={ui.field}><span className={ui.label}>{t('chores.behavior')}</span><select aria-label={t('chores.behavior')} className={ui.input} value={resetDaily ? 'reset' : 'persist'} onChange={e => setResetDaily(e.target.value === 'reset')} disabled={saving}><option value="persist">{t('chores.untilDone')}</option><option value="reset">{t('chores.startFresh')}</option></select><span className={ui.help}>{t('chores.behaviorHelp')}</span></label>
            {user.role === 'admin' && <>
              <label className={ui.check}><input type="checkbox" checked={approval} onChange={e => setApproval(e.target.checked)} disabled={saving} />{t('admin.createChore.field.requiresApproval')}</label>
              <label className={ui.check}><input type="checkbox" checked={photo} onChange={e => setPhoto(e.target.checked)} disabled={saving} />{t('admin.createChore.field.requiresPhoto')}</label>
            </>}
          </div>
        </details>
        {error && <p className={ui.msgError} role="alert">{error}</p>}
        {message && <p className={ui.msg} role="status">{message}</p>}
        <div className={ui.actionsEnd}>
          <button className={ui.btn} type="button" disabled={saving || !title.trim()} onClick={() => void save(true)}>{t('chores.saveAnother')}</button>
          <button className={ui.btnPrimary} type="submit" disabled={saving || !title.trim()}>{t(saving ? 'chores.saving' : 'chores.save')}</button>
        </div>
      </form>
    </Modal>
  );
}
