// @vitest-environment jsdom
import { afterEach, beforeAll, beforeEach, expect, it, vi } from 'vitest';
import { cleanup, render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { api } from '../api';
import i18n from '../i18n';
import type { Chore, ScheduledChore } from '../types';
import { ChoreDetails } from './ChoreDetails';

const auth = vi.hoisted(() => ({ user: { role: 'admin' } }));
vi.mock('../AuthContext', () => ({ useAuth: () => auth }));
vi.mock('../api', () => ({ api: { chores: { get: vi.fn(), history: vi.fn() }, users: { list: vi.fn() } } }));
vi.mock('../hooks/useAIStatus', () => ({ useAIStatus: () => ({ ai: { configured: false }, tts: { configured: false } }) }));
vi.mock('./admin/ScheduleManager', () => ({ ScheduleManager: () => null }));
vi.mock('./admin/TriggerManager', () => ({ TriggerManager: () => null }));
const chore: Chore = { id: 1, title: 'Clean shelf', description: 'Use a soft cloth', category: 'independent', points_value: 0, missed_penalty_value: 0, requires_approval: false, requires_photo: false };
const scheduled: ScheduledChore = { ...chore, chore_id: chore.id, schedule_id: 2, assignment_type: 'individual', expiry_penalty: 'block', expiry_penalty_value: 0, available: true, expired: false, completed: false, date: '2026-10-23' };
beforeAll(async () => { await i18n.changeLanguage('en'); });
beforeEach(() => {
  vi.clearAllMocks(); auth.user.role = 'admin';
  vi.mocked(api.chores.get).mockResolvedValue(chore);
  vi.mocked(api.chores.history).mockResolvedValue([]);
  vi.mocked(api.users.list).mockResolvedValue([]);
});
afterEach(cleanup);

it('opens the editor directly and returns from history before closing the popup', async () => {
  const interaction = userEvent.setup(); const close = vi.fn();
  render(<ChoreDetails chore={scheduled} onClose={close} />);
  expect(await screen.findByLabelText('Title')).toHaveProperty('value', 'Clean shelf');
  expect(screen.queryByRole('button', { name: 'Edit chore and schedule' })).toBeNull();
  expect(api.chores.history).not.toHaveBeenCalled();
  await interaction.click(screen.getByRole('button', { name: 'Chore history' }));
  expect(await screen.findByText('No history yet.')).toBeTruthy();
  await interaction.click(screen.getByRole('button', { name: 'Close' }));
  expect(close).not.toHaveBeenCalled();
  expect(screen.getByLabelText('Title')).toHaveProperty('value', 'Clean shelf');
  await interaction.click(screen.getByRole('button', { name: 'Close' }));
  expect(close).toHaveBeenCalledOnce();
});

it('keeps chore details read-only for a child', async () => {
  auth.user.role = 'child';
  render(<ChoreDetails chore={scheduled} onClose={vi.fn()} />);
  expect(await screen.findByText('No history yet.')).toBeTruthy();
  expect(screen.queryByLabelText('Title')).toBeNull();
  expect(api.chores.get).not.toHaveBeenCalled();
});
