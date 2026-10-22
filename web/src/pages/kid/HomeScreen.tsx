import React from 'react';
import { useTranslation } from 'react-i18next';
import type { User } from '../../types';
import { HomeBacklogTab } from '../../components/admin/HomeBacklogTab';
import type { KidData } from './useKidData';
import s from './HomeScreen.module.css';

interface Props { user: User; data: KidData; topBar: React.ReactNode; refreshKey: number }

/** The Home tab shares its editable backlog list with Manage → Chores → Home. */
export function HomeScreen({ user, data, topBar, refreshKey }: Props) {
  const { t } = useTranslation();

  return (
    <div className={s.page}>
      <header className={s.hero}>
        {topBar}
        <h1>{t('design.tabs.home')}</h1>
        <p>{t('kid.home.subtitle', { name: user.name.split(' ')[0] })}</p>
      </header>
      <main className={s.content}>
        <HomeBacklogTab
          embedded
          refreshKey={refreshKey}
          onChoreChanged={() => { void data.refreshAll(); }}
        />
      </main>
    </div>
  );
}
