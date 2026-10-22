import React, { useCallback, useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { api } from '../../api';
import type { Chore, HomeBacklogItem, User } from '../../types';
import { localDateStr } from '../../utils';
import { Avatar, catFromCategory, Icon, isPersonColor } from '../../design';
import EditChoreModal from '../EditChoreModal/EditChoreModal';
import { HomeAssigneePicker } from '../HomeAssigneePicker';
import { ScheduleManager } from './ScheduleManager';
import { TriggerManager } from './TriggerManager';
import { IconWell } from './pickers';
import ui from './ui.module.css';
import s from './HomeBacklogTab.module.css';

export const HomeBacklogTab: React.FC<{ embedded?: boolean; refreshKey?: number; onChoreChanged?: () => void }> = ({ embedded = false, refreshKey = 0, onChoreChanged }) => {
  const { t, i18n } = useTranslation();
  const [items, setItems] = useState<HomeBacklogItem[]>([]);
  const [history, setHistory] = useState<HomeBacklogItem[]>([]);
  const [users, setUsers] = useState<User[]>([]);
  const [editingChore, setEditingChore] = useState<Chore | null>(null);
  const [loading, setLoading] = useState(true);
  const [completing, setCompleting] = useState<number | null>(null);
  const [assigning, setAssigning] = useState<number | null>(null);
  const [error, setError] = useState('');
  const [historyExpanded, setHistoryExpanded] = useState(false);

  const load = useCallback(async () => {
    try {
      const [current, previous] = await Promise.all([
        api.chores.listHomeBacklog(),
        api.chores.listHomeBacklogHistory(),
      ]);
      setItems(current);
      setHistory(previous);
      setError('');
    } catch {
      setError(t('admin.homeBacklog.loadError'));
    } finally {
      setLoading(false);
    }
  }, [t]);

  useEffect(() => { void load(); }, [load, refreshKey]);

  useEffect(() => { api.users.list().then(setUsers).catch(() => {}); }, []);

  const complete = async (item: HomeBacklogItem) => {
    setCompleting(item.chore_id);
    setError('');
    try {
      await api.chores.completeHomeBacklog(item.chore_id, localDateStr(new Date()));
      await load();
    } catch {
      setError(t('admin.homeBacklog.completeError'));
    } finally {
      setCompleting(null);
    }
  };

  const uncomplete = async (item: HomeBacklogItem) => {
    setCompleting(item.chore_id);
    setError('');
    try {
      if (!item.completed_by) throw new Error('Completion recipient is missing');
      if (item.completion_id) await api.chores.uncompleteHomeBacklogCompletion(item.completion_id, item.completed_by);
      else await api.chores.uncompleteHomeBacklog(item.chore_id, item.completed_by);
      await load();
    } catch {
      setError(t('admin.homeBacklog.uncompleteError'));
    } finally {
      setCompleting(null);
    }
  };

  const repeat = async (item: HomeBacklogItem) => {
    setCompleting(item.chore_id);
    setError('');
    try {
      await api.chores.repeatHomeBacklog(item.chore_id);
      await load();
    } catch {
      setError(t('admin.homeBacklog.repeatError'));
    } finally {
      setCompleting(null);
    }
  };

  const assign = async (item: HomeBacklogItem, userID: number | null | 'all') => {
    setAssigning(item.chore_id);
    setError('');
    try {
      if (userID === null) await api.chores.unassignHomeBacklog(item.chore_id);
      else await api.chores.assignHomeBacklog(item.chore_id, userID);
      await load();
    } catch {
      setError(t('admin.homeBacklog.assignError'));
    } finally {
      setAssigning(null);
    }
  };

  const edit = async (item: HomeBacklogItem) => {
    try {
      setEditingChore(await api.chores.get(item.chore_id));
    } catch {
      setError(t('admin.homeBacklog.loadError'));
    }
  };

  const open = items.filter(item => !item.completed_at);
  const done = items.filter(item => !!item.completed_at);
  const date = (value?: string | null) => value
    ? new Intl.DateTimeFormat(i18n.language, { dateStyle: 'medium', timeStyle: 'short' }).format(new Date(value))
    : '';

  const rows = (list: HomeBacklogItem[], completed: boolean) => (
    <div className={`${ui.list} ${s.list}`}>
      {list.map(item => (
        <div className={`${ui.row} ${s.row}`} key={`${item.chore_id}-${item.completion_id ?? 'current'}`}>
          <button
            type="button"
            className={`${ui.rowButton} ${s.edit}`}
            onClick={() => void edit(item)}
            title={t('admin.homeBacklog.edit')}
          >
            <IconWell icon={item.icon} cat={catFromCategory('home')} />
            <span className={ui.rowMain}>
              <span className={ui.rowTitle}>{item.title}</span>
              {item.description && <span className={`${ui.rowDesc} ${s.description}`}>{item.description}</span>}
              <span className={ui.rowMeta}>
                <span className={s.points}><Icon name="star" /> {t('admin.choresTab.points', { count: item.completed_at ? item.points_awarded ?? item.points_value : item.points_value })}</span>
                {item.estimated_minutes ? <span><Icon name="clock" /> {t('admin.choresTab.minutes', { count: item.estimated_minutes })}</span> : null}
                {item.assigned_to_all
                  ? <span>{t('admin.homeBacklog.assignedAll')}</span>
                  : item.assigned_to_name && <span>{t('admin.homeBacklog.assignedTo', { name: item.assigned_to_name })}</span>}
                {completed && (
                  <span>{t('admin.homeBacklog.completedBy', { name: item.completed_by_name || t('admin.homeBacklog.unknown'), date: date(item.completed_at) })}</span>
                )}
              </span>
            </span>
            <Icon name="chev" />
          </button>
          <div className={`${ui.rowActions} ${s.actions}`}>
            {completed && (
              <span
                title={item.completed_by_name || t('admin.homeBacklog.unknown')}
                aria-label={item.completed_by_name || t('admin.homeBacklog.unknown')}
              >
                <Avatar
                  name={item.completed_by_name || t('admin.homeBacklog.unknown')}
                  color={isPersonColor(item.completed_by_color) ? item.completed_by_color : null}
                  src={users.find(user => user.id === item.completed_by)?.avatar_url}
                  size="sm"
                />
              </span>
            )}
            {!completed && users.length > 0 && (
              <HomeAssigneePicker
                users={users}
                assignedTo={item.assigned_to}
                assignedToAll={item.assigned_to_all}
                disabled={assigning === item.chore_id}
                label={t('admin.homeBacklog.assignLabel', { chore: item.title })}
                unassignedLabel={t('admin.homeBacklog.unassigned')}
                allAdminsLabel={t('admin.homeBacklog.allAdmins')}
                onSelect={userID => void assign(item, userID)}
              />
            )}
            {completed ? (
              <>
                {!item.history && (
                  <button
                    type="button"
                    className={ui.iconBtn}
                    disabled={completing === item.chore_id}
                    onClick={() => void repeat(item)}
                    title={t('admin.homeBacklog.repeat')}
                    aria-label={t('admin.homeBacklog.repeat')}
                  >
                    <Icon name="plus" />
                  </button>
                )}
                <button
                  type="button"
                  className={ui.iconBtn}
                  disabled={completing === item.chore_id}
                  onClick={() => void uncomplete(item)}
                  title={t('admin.homeBacklog.uncheck')}
                  aria-label={t('admin.homeBacklog.uncheck')}
                >
                  <Icon name="back" />
                </button>
              </>
            ) : (
              <button
                type="button"
                className={ui.iconBtn}
                disabled={completing === item.chore_id}
                onClick={() => void complete(item)}
                title={completing === item.chore_id ? t('admin.homeBacklog.completing') : t('admin.homeBacklog.complete')}
                aria-label={completing === item.chore_id ? t('admin.homeBacklog.completing') : t('admin.homeBacklog.complete')}
              >
                <Icon name="check" />
              </button>
            )}
          </div>
        </div>
      ))}
    </div>
  );

  return (
    <div className={ui.page}>
      {editingChore && (
        <EditChoreModal
          key={editingChore.id}
          chore={editingChore}
          isOpen
          onClose={() => { setEditingChore(null); void load(); onChoreChanged?.(); }}
          onSaved={() => { void load(); onChoreChanged?.(); }}
          users={users}
          renderSchedules={(choreId, people) => <ScheduleManager choreId={choreId} users={people} />}
          renderTriggers={(choreId, people) => <TriggerManager choreId={choreId} users={people} />}
        />
      )}
      {!embedded && <div className={ui.pageHead}>
        <div>
          <h2 className={ui.pageTitle}>{t('admin.homeBacklog.heading')}</h2>
          <p className={ui.pageSub}>{t('admin.homeBacklog.subtitle')}</p>
        </div>
      </div>}
      {error && <p role="alert" className={ui.msg}>{error}</p>}
      {loading ? null : open.length ? (
        <section className={ui.section}>
          <h3 className={ui.sectionTitle}>{t('admin.homeBacklog.openHeading', { count: open.length })}</h3>
          {rows(open, false)}
        </section>
      ) : <div className={ui.empty}><p>{t('admin.homeBacklog.empty')}</p></div>}
      {!!done.length || !!history.length ? (
        <section className={ui.section}>
          <h3 className={`${ui.sectionTitle} ${s.desktopHeading}`}>{t('admin.homeBacklog.historyHeading')}</h3>
          <button
            type="button"
            className={s.historyToggle}
            aria-expanded={historyExpanded}
            aria-controls="home-backlog-history"
            onClick={() => setHistoryExpanded(expanded => !expanded)}
          >
            {t('admin.homeBacklog.historyHeading')} ({done.length + history.length})
            <Icon name={historyExpanded ? 'chev' : 'plus'} />
          </button>
          <div id="home-backlog-history" className={historyExpanded ? s.history : `${s.history} ${s.historyCollapsed}`}>
            {rows([...done, ...history], true)}
          </div>
        </section>
      ) : null}
    </div>
  );
};
