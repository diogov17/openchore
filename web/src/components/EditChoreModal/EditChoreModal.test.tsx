// @vitest-environment jsdom
import { afterEach, beforeAll, expect, it, vi } from 'vitest';
import { cleanup, render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import i18n from '../../i18n';
import type { Chore } from '../../types';
import EditChoreModal from './EditChoreModal';
import { api } from '../../api';

vi.mock('../../api', () => ({ api: { chores: { history: vi.fn().mockResolvedValue([]), delete: vi.fn() } } }));
vi.mock('../../hooks/useAIStatus', () => ({ useAIStatus: () => ({ ai: { configured: false }, tts: { configured: false } }) }));
beforeAll(async () => { await i18n.changeLanguage('en'); });
afterEach(() => { cleanup(); vi.restoreAllMocks(); vi.clearAllMocks(); });

it('replaces the editor with history and restores the draft and focus when returning', async () => {
  const interaction = userEvent.setup();
  const close = vi.fn();
  const chore: Chore = { id: 1, title: 'Clean shelf', description: '', category: 'independent', points_value: 0, missed_penalty_value: 0, requires_approval: false, requires_photo: false };
  render(<EditChoreModal chore={chore} users={[]} isOpen onClose={close} onSaved={vi.fn()} renderSchedules={() => <input aria-label="Schedule draft" defaultValue="Monday" />} renderTriggers={() => null} />);
  const history = screen.getByRole('button', { name: 'Chore history' });
  expect(history.textContent).toBe('');
  expect(history.nextElementSibling).toBe(screen.getByRole('button', { name: 'Close' }));
  await interaction.clear(screen.getByLabelText('Title'));
  await interaction.type(screen.getByLabelText('Title'), 'Clean all shelves');
  await interaction.type(screen.getByLabelText('Schedule draft'), ' evening');
  await interaction.click(screen.getByRole('button', { name: 'Chore history' }));
  expect(screen.getAllByRole('dialog')).toHaveLength(1);
  expect(screen.queryByRole('textbox', { name: 'Title' })).toBeNull();
  expect(await screen.findByText('No history yet.')).toBeTruthy();
  // The focus trap must skip the hidden editor's inputs and buttons.
  await interaction.tab();
  expect(document.activeElement).toBe(screen.getByRole('button', { name: 'Close' }));
  await interaction.click(screen.getByRole('button', { name: 'Close' }));
  expect(close).not.toHaveBeenCalled();
  expect((screen.getByRole('textbox', { name: 'Title' }) as HTMLInputElement).value).toBe('Clean all shelves');
  expect((screen.getByLabelText('Schedule draft') as HTMLInputElement).value).toBe('Monday evening');
  expect(document.activeElement).toBe(screen.getByRole('button', { name: 'Chore history' }));
  await interaction.click(screen.getByRole('button', { name: 'Chore history' }));
  await interaction.click(screen.getByRole('button', { name: 'Back to chore' }));
  expect(close).not.toHaveBeenCalled();
  await interaction.click(screen.getByRole('button', { name: 'Chore history' }));
  await interaction.keyboard('{Escape}');
  expect(close).not.toHaveBeenCalled();
  expect(screen.getByRole('textbox', { name: 'Title' })).toBeTruthy();
  await interaction.keyboard('{Escape}');
  expect(close).toHaveBeenCalledOnce();
});

it('places Delete before History and deletes only the open chore after confirmation', async () => {
  const interaction = userEvent.setup(); const close = vi.fn(); const changed = vi.fn();
  const confirm = vi.spyOn(window, 'confirm').mockReturnValue(false);
  vi.mocked(api.chores.delete).mockResolvedValue({});
  const chore: Chore = { id: 42, title: 'Clean shelf', description: '', category: 'independent', points_value: 0, missed_penalty_value: 0, requires_approval: false, requires_photo: false };
  render(<EditChoreModal chore={chore} users={[]} isOpen onClose={close} onSaved={changed} renderSchedules={() => null} renderTriggers={() => null} />);
  const button = screen.getByRole('button', { name: 'Delete Clean shelf' });
  expect(button.nextElementSibling).toBe(screen.getByRole('button', { name: 'Chore history' }));
  await interaction.click(button);
  expect(api.chores.delete).not.toHaveBeenCalled();
  expect(close).not.toHaveBeenCalled();
  confirm.mockReturnValue(true);
  await interaction.click(button);
  expect(api.chores.delete).toHaveBeenCalledExactlyOnceWith(42);
  expect(changed).toHaveBeenCalledOnce();
  expect(close).toHaveBeenCalledOnce();
});

it('keeps the editor open and shows an error when deletion fails', async () => {
  const interaction = userEvent.setup(); const close = vi.fn();
  vi.spyOn(window, 'confirm').mockReturnValue(true);
  vi.mocked(api.chores.delete).mockRejectedValue(new Error('Connection lost'));
  const chore: Chore = { id: 42, title: 'Clean shelf', description: '', category: 'independent', points_value: 0, missed_penalty_value: 0, requires_approval: false, requires_photo: false };
  render(<EditChoreModal chore={chore} users={[]} isOpen onClose={close} onSaved={vi.fn()} renderSchedules={() => null} renderTriggers={() => null} />);
  await interaction.click(screen.getByRole('button', { name: 'Delete Clean shelf' }));
  expect(await screen.findByRole('alert')).toHaveProperty('textContent', 'Connection lost');
  expect(close).not.toHaveBeenCalled();
  expect(screen.getByLabelText('Title')).toHaveProperty('value', 'Clean shelf');
});
