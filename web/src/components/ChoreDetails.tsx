import { useState } from 'react';
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
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const edit = async () => {
    setBusy(true); setError('');
    try { const [loadedChore, people] = await Promise.all([api.chores.get(chore.chore_id), api.users.list()]); setUsers(people); setEditing(loadedChore); }
    catch { setError(t('kid.errors.couldntSave')); }
    finally { setBusy(false); }
  };
  if (editing) return <EditChoreModal chore={editing} users={users} isOpen onClose={() => { onChanged?.(); onClose(); }} onSaved={() => onChanged?.()} renderSchedules={(id, people) => <ScheduleManager choreId={id} users={people} />} renderTriggers={(id, people) => <TriggerManager choreId={id} users={people} />} />;
  return <Modal isOpen onClose={onClose} title={chore.title} maxWidth="600px">
    <div className={ui.page}>
      {chore.description && <p className={ui.sectionDesc}>{chore.description}</p>}
      {user?.role === 'admin' && <button type="button" className={ui.btn} onClick={() => void edit()} disabled={busy}>{t('chores.editOptions')}</button>}
      {error && <p role="alert">{error}</p>}
      <ChoreHistory choreId={chore.chore_id} />
    </div>
  </Modal>;
}
