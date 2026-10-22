// @vitest-environment jsdom
import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from 'vitest';
import { act, cleanup, renderHook, waitFor } from '@testing-library/react';
import { api, APIError } from '../../api';
import i18n from '../../i18n';
import type { ScheduledChore, User } from '../../types';
import { localDateStr } from '../../utils';
import { useKidData } from './useKidData';

vi.mock('../../api', async importOriginal => {
  const original = await importOriginal<typeof import('../../api')>();
  return { ...original, api: {
    ...original.api,
    users: { ...original.api.users, getChores: vi.fn().mockResolvedValue([]) },
    chores: { ...original.api.chores, listMyAssignedHome: vi.fn().mockResolvedValue([]), complete: vi.fn().mockResolvedValue({}), uncomplete: vi.fn().mockResolvedValue({}) },
    streaks: { ...original.api.streaks, getForUser: vi.fn().mockResolvedValue({}) },
    points: { ...original.api.points, getForUser: vi.fn().mockResolvedValue({ balance: 0, active_commitments: [] }) },
  } };
});

const user: User = { id: 7, name: 'Alex', role: 'child', avatar_url: '', theme: 'sunroom', paused: false, has_pin: false, auth_providers: [], created_at: '' };
function chore(offset: number): ScheduledChore {
  const date = new Date(); date.setDate(date.getDate() + offset);
  return { schedule_id: 42, chore_id: 8, title: 'Read', description: '', category: 'independent', points_value: 0, missed_penalty_value: 0, requires_approval: false, requires_photo: false, assignment_type: 'individual', expiry_penalty: 'block', expiry_penalty_value: 0, available: true, expired: false, completed: false, date: localDateStr(date) };
}
beforeAll(async () => { await i18n.changeLanguage('en'); });
beforeEach(() => { vi.clearAllMocks(); vi.mocked(api.chores.complete).mockResolvedValue({}); });
afterEach(cleanup);

describe('Week completion actions', () => {
  it('sends the original date for an earlier chore and refreshes Week', async () => {
    const { result } = renderHook(() => useKidData(user, { week: true, rewards: false }));
    await waitFor(() => expect(result.current.chores).not.toBeNull());
    const past = chore(-1);
    await act(async () => { expect(await result.current.toggle(past)).toEqual({ finished: true }); });
    expect(api.chores.complete).toHaveBeenCalledWith(42, past.date);
    expect(api.users.getChores).toHaveBeenCalledWith(7, 'weekly', result.current.today);
  });
  it('keeps the earlier date when photo proof is needed', async () => {
    vi.mocked(api.chores.complete).mockRejectedValue(new APIError('photo required', 400, {}));
    const { result } = renderHook(() => useKidData(user, { week: true, rewards: false }));
    await waitFor(() => expect(result.current.chores).not.toBeNull());
    const past = { ...chore(-1), requires_photo: true, photo_source: 'child' as const };
    await act(async () => { expect(await result.current.toggle(past)).toEqual({ needsPhoto: true }); });
    expect(api.chores.complete).toHaveBeenCalledWith(42, past.date);
  });
  it('ignores future chores', async () => {
    const { result } = renderHook(() => useKidData(user, { week: true, rewards: false }));
    await waitFor(() => expect(result.current.chores).not.toBeNull());
    await act(async () => { expect(await result.current.toggle(chore(1))).toEqual({}); });
    expect(api.chores.complete).not.toHaveBeenCalled();
  });
});
