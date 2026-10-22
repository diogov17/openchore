import { useState } from 'react';
import { Avatar } from '../design';
import type { User } from '../types';
import styles from './HomeAssigneePicker.module.css';

interface Props {
  users: User[];
  assignedTo?: number | null;
  assignedToAll?: boolean;
  label: string;
  unassignedLabel: string;
  allAdminsLabel: string;
  disabled?: boolean;
  onSelect: (userID: number | null | 'all') => void;
}

export function HomeAssigneePicker({ users, assignedTo, assignedToAll = false, label, unassignedLabel, allAdminsLabel, disabled, onSelect }: Props) {
  const [open, setOpen] = useState(false);
  const selected = users.find(person => person.id === assignedTo);

  const choose = (userID: number | null | 'all') => {
    setOpen(false);
    onSelect(userID);
  };

  return (
    <div className={styles.picker}>
      <button
        type="button"
        className={styles.trigger}
        aria-label={label}
        title={assignedToAll ? allAdminsLabel : selected?.name || unassignedLabel}
        aria-haspopup="true"
        aria-expanded={open}
        disabled={disabled}
        onClick={() => setOpen(value => !value)}
      >
        {assignedToAll
          ? <span className={styles.allMark} aria-hidden="true">A</span>
          : selected
            ? <Avatar name={selected.name} color={selected.color} src={selected.avatar_url} size="sm" />
            : <span className={styles.dash} aria-hidden="true">—</span>}
      </button>
      {open && (
        <div className={styles.menu} role="group" aria-label={label}>
          <button
            type="button"
            className={`${styles.option} ${!selected && !assignedToAll ? styles.selected : ''}`}
            aria-label={unassignedLabel}
            title={unassignedLabel}
            onClick={() => choose(null)}
          >
            <span className={styles.dash} aria-hidden="true">—</span>
          </button>
          <button
            type="button"
            className={`${styles.option} ${assignedToAll ? styles.selected : ''}`}
            aria-label={allAdminsLabel}
            title={allAdminsLabel}
            aria-pressed={assignedToAll}
            onClick={() => choose('all')}
          >
            <span className={styles.allMark} aria-hidden="true">A</span>
          </button>
          {users.map(person => (
            <button
              type="button"
              key={person.id}
              className={`${styles.option} ${person.id === assignedTo ? styles.selected : ''}`}
              aria-label={person.name}
              title={person.name}
              aria-pressed={person.id === assignedTo}
              onClick={() => choose(person.id)}
            >
              <Avatar name={person.name} color={person.color} src={person.avatar_url} size="sm" />
            </button>
          ))}
        </div>
      )}
    </div>
  );
}
