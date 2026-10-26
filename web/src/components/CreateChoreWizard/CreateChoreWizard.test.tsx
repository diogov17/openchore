// @vitest-environment jsdom
import { afterEach, beforeAll, beforeEach, expect, it, vi } from 'vitest';
import { cleanup, render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import i18n from '../../i18n';
import { api } from '../../api';
import type { Chore, User } from '../../types';
import CreateChoreWizard from './CreateChoreWizard';

vi.mock('../../AuthContext', () => ({ useAuth: () => ({ user: { id: 7 } }) }));
vi.mock('../../hooks/useAIStatus', () => ({ useAIStatus: () => ({ ai: { configured: false }, tts: { configured: false } }) }));
vi.mock('../../api', () => ({ api: { chores: { create: vi.fn(), createSchedule: vi.fn() }, users: { list: vi.fn() }, googleCalendar: { status: vi.fn(), add: vi.fn() } } }));
const user: User = { id: 7, name: 'Alex', role: 'admin', avatar_url: '', theme: 'sunroom', paused: false, has_pin: false, auth_providers: [], created_at: '' };
beforeAll(async () => { await i18n.changeLanguage('en'); });
beforeEach(() => {
  vi.clearAllMocks();
  vi.mocked(api.chores.create).mockResolvedValue({ id: 42 } as Chore);
  vi.mocked(api.chores.createSchedule).mockResolvedValue({} as Awaited<ReturnType<typeof api.chores.createSchedule>>);
  vi.mocked(api.users.list).mockResolvedValue([user]);
  vi.mocked(api.googleCalendar.status).mockResolvedValue({ configured: true, connected: true, calendar_id: 'primary', calendars: [{ id: 'alex@example.com', summary: 'Personal', primary: true }] });
  vi.mocked(api.googleCalendar.add).mockResolvedValue({ added: 1 });
});
afterEach(cleanup);

it('automatically adds the saved chore and retries Calendar failures without duplicating chores or schedules', async () => {
  const interaction = userEvent.setup(); const close = vi.fn(); const saved = vi.fn();
  render(<CreateChoreWizard isOpen todayUser={user} users={[user]} onClose={close} onComplete={saved} />);
  await interaction.type(screen.getByLabelText('Title *'), 'Appointment');
  await interaction.click(screen.getByRole('button', { name: 'Next' }));
  await interaction.click(screen.getByRole('button', { name: 'Next' }));
  const toggle = await screen.findByRole('checkbox', { name: 'Add to Google Calendar' });
  await interaction.click(toggle);
  vi.mocked(api.googleCalendar.add).mockRejectedValueOnce(new Error('Google is unavailable'));
  await interaction.click(screen.getByRole('button', { name: 'Save' }));
  expect(await screen.findByRole('alert')).toHaveProperty('textContent', 'Google is unavailable');
  expect(saved).not.toHaveBeenCalled();
  expect(close).not.toHaveBeenCalled();
  await interaction.click(screen.getByRole('button', { name: 'Retry calendar add' }));
  expect(api.chores.create).toHaveBeenCalledTimes(1);
  expect(api.chores.createSchedule).toHaveBeenCalledTimes(1);
  expect(api.googleCalendar.add).toHaveBeenCalledTimes(2);
  expect(api.googleCalendar.add).toHaveBeenLastCalledWith(42);
  expect(close).toHaveBeenCalledOnce();
  expect(saved).toHaveBeenCalledWith(42);
});

it('uses the Manage form while allowing a name-only save from Today', async () => {
  const interaction = userEvent.setup(); const close = vi.fn(); const saved = vi.fn();
  render(<CreateChoreWizard isOpen todayUser={user} users={[user]} onClose={close} onComplete={saved} />);
  expect(screen.queryByRole('radio')).toBeNull();
  expect(screen.getAllByRole('textbox')).toHaveLength(1);
  await interaction.type(screen.getByLabelText('Title *'), 'Fix shelf');
  await interaction.click(screen.getByRole('button', { name: 'Save' }));
  expect(api.chores.create).toHaveBeenCalledWith(expect.objectContaining({ title: 'Fix shelf', category: 'independent', points_value: 0, reset_daily: false }));
  expect(api.chores.createSchedule).toHaveBeenCalledWith(42, expect.objectContaining({ assigned_to: 7, specific_date: expect.stringMatching(/^\d{4}-\d{2}-\d{2}$/), assignment_type: 'individual' }));
  expect(saved).toHaveBeenCalledWith(42);
  expect(close).toHaveBeenCalledOnce();
});

it('moves from title to details, then schedule and review, preserving values when going back', async () => {
  const interaction = userEvent.setup();
  render(<CreateChoreWizard isOpen users={[user]} onClose={vi.fn()} onComplete={vi.fn()} />);
  expect(screen.getByRole('button', { name: 'Next' })).toHaveProperty('disabled', true);
  await interaction.type(screen.getByLabelText('Title *'), 'Clean shelves');
  await interaction.click(screen.getByRole('button', { name: 'Next' }));
  expect(screen.queryByLabelText('Title *')).toBeNull();
  expect(screen.getByRole('button', { name: 'About chore types' })).toBeTruthy();
  await interaction.type(screen.getByLabelText('Description'), 'Use the soft cloth');
  await interaction.click(screen.getByRole('button', { name: 'Back' }));
  expect(screen.getByLabelText('Title *')).toHaveProperty('value', 'Clean shelves');
  await interaction.click(screen.getByRole('button', { name: 'Next' }));
  expect(screen.getByLabelText('Description')).toHaveProperty('value', 'Use the soft cloth');
  await interaction.click(screen.getByRole('button', { name: 'Next' }));
  expect(screen.getByLabelText('Schedule type')).toBeTruthy();
  await interaction.click(screen.getByRole('button', { name: 'Skip' }));
  expect(screen.getByRole('button', { name: 'Create Chore' })).toBeTruthy();
  const edits = screen.getAllByRole('button', { name: 'Edit' });
  await interaction.click(edits[1]);
  expect(screen.getByLabelText('Schedule type')).toBeTruthy();
});

it('keeps the Today form open for another chore and preserves a failed draft', async () => {
  const interaction = userEvent.setup(); const close = vi.fn();
  render(<CreateChoreWizard isOpen todayUser={user} users={[user]} onClose={close} onComplete={vi.fn()} />);
  await interaction.type(screen.getByLabelText('Title *'), 'First chore');
  await interaction.click(screen.getByRole('button', { name: 'Save and add another' }));
  expect(await screen.findByRole('status')).toBeTruthy();
  expect(close).not.toHaveBeenCalled();
  expect(screen.getByLabelText('Title *')).toHaveProperty('value', '');
  vi.mocked(api.chores.create).mockRejectedValueOnce(new Error('Connection lost'));
  await interaction.type(screen.getByLabelText('Title *'), 'Second chore');
  await interaction.click(screen.getByRole('button', { name: 'Save' }));
  expect(await screen.findByRole('alert')).toHaveProperty('textContent', 'Connection lost');
  expect(screen.getByLabelText('Title *')).toHaveProperty('value', 'Second chore');
  expect(close).not.toHaveBeenCalled();
});
