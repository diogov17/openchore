import { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useAuth } from '../AuthContext';
import { api } from '../api';
import type { Chore, ScheduledChore, User } from '../types';
import Modal from './Modal/Modal';
import EditChoreModal from './EditChoreModal/EditChoreModal';
import { ScheduleManager } from './admin/ScheduleManager';
import { TriggerManager } from './admin/TriggerManager';
import { ChoreHistory } from './ChoreHistory';
import ui from './admin/ui.module.css';

export function ChoreDetails({ chore, onClose, onChanged }: { chore: ScheduledChore; onClose: () => void; onChanged?: () => void }) {
  const { t } = useTranslation();
  const { user } = useAuth();
  const [editing, setEditing] = useState<Chore | null>(null);
  const [users, setUsers] = useState<User[]>([]);
  const [attempt, setAttempt] = useState(0);
  const [error, setError] = useState('');
  const canEdit = user?.role === 'admin';
  useEffect(() => {
    if (!canEdit) return;
    let live = true;
    setEditing(null); setError('');
    Promise.all([api.chores.get(chore.chore_id), api.users.list()])
      .then(([loadedChore, people]) => { if (live) { setUsers(people); setEditing(loadedChore); } })
      .catch(() => { if (live) setError(t('kid.errors.couldntSave')); });
    return () => { live = false; };
  }, [canEdit, chore.chore_id, attempt, t]);
  if (canEdit && editing) return <EditChoreModal chore={editing} users={users} isOpen onClose={() => { onChanged?.(); onClose(); }} onSaved={() => onChanged?.()} renderSchedules={(id, people) => <ScheduleManager choreId={id} users={people} />} renderTriggers={(id, people) => <TriggerManager choreId={id} users={people} />} />;
  return <Modal isOpen onClose={onClose} title={chore.title} maxWidth="600px">
    <div className={ui.page}>
      {canEdit ? error ? <><p role="alert">{error}</p><button type="button" className={ui.btn} onClick={() => setAttempt(n => n + 1)}>{t('chores.editOptions')}</button></> : <p role="status">{t('chores.loading')}</p> : <>
        {chore.description && <p className={ui.sectionDesc}>{chore.description}</p>}
        <ChoreHistory choreId={chore.chore_id} />
      </>}
    </div>
  </Modal>;
}
