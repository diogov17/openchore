// @vitest-environment jsdom
import { afterEach, beforeAll, describe, expect, it } from 'vitest';
import { cleanup, render, screen } from '@testing-library/react';
import i18n from '../../i18n';
import type { HomeBacklogItem, User } from '../../types';
import type { KidData } from './useKidData';
import { TodayScreen } from './TodayScreen';

beforeAll(async () => {
  await i18n.changeLanguage('en');
});
afterEach(cleanup);

describe('TodayScreen', () => {
  it('shows the configured points for an incomplete assigned Home chore', () => {
    const today = '2026-10-19';
    const item: HomeBacklogItem = {
      chore_id: 42,
      title: 'Set the table',
      description: '',
      points_value: 5,
      points_awarded: 0,
      completed_at: null,
    };
    const data = {
      today,
      chores: [],
      homeBacklog: [],
      assignedHome: [item],
      points: null,
      togglingIds: new Set<number>(),
      people: [],
    } as unknown as KidData;
    const user: User = {
      id: 1,
      name: 'Alex',
      avatar_url: '',
      role: 'admin',
      theme: 'sunroom',
      paused: false,
      has_pin: false,
      auth_providers: [],
      created_at: today,
    };

    render(
      <TodayScreen
        user={user}
        data={data}
        now={new Date(`${today}T10:00:00`)}
        topBar={null}
        viewOf={() => ({
          cat: 'daily', state: 'todo', urgent: false, photo: false, canToggle: true,
          retry: false, needsPhotoProof: false,
        })}
        tts={false}
        onToggle={() => {}}
        onPhoto={() => {}}
        onSpeak={() => {}}
      />,
    );

    expect(screen.getByText('Set the table')).toBeTruthy();
    expect(screen.getByText('5 pts')).toBeTruthy();
    expect(screen.queryByText('0 pts')).toBeNull();
  });
});
