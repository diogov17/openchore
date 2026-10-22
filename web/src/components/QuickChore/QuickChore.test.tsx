// @vitest-environment jsdom
import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { api } from '../../api';
import i18n from '../../i18n';
import type { Chore, User } from '../../types';
import { QuickChore } from './QuickChore';

vi.mock('../../api', () => ({ api: { chores: { quickAdd: vi.fn() }, users: { list: vi.fn() } } }));
const user: User = { id: 7, name: 'Alex', role: 'admin', avatar_url: '', theme: 'sunroom', paused: false, has_pin: false, auth_providers: [], created_at: '' };
beforeAll(async () => { await i18n.changeLanguage('en'); });
beforeEach(() => { vi.mocked(api.chores.quickAdd).mockReset().mockResolvedValue({ id: 42 } as Chore); vi.mocked(api.users.list).mockResolvedValue([user]); });
afterEach(cleanup);

describe('quick add', () => {
  it('saves a name-only independent chore for the current person and closes', async () => {
    const close = vi.fn(); const saved = vi.fn(); const interaction = userEvent.setup();
    render(<QuickChore user={user} onClose={close} onSaved={saved} />);
    await interaction.type(screen.getByLabelText('Chore name'), 'Fix shelf');
    await interaction.click(screen.getByRole('button', { name: 'Save' }));
    expect(api.chores.quickAdd).toHaveBeenCalledWith(expect.objectContaining({ title: 'Fix shelf', assigned_to: 7, category: 'independent', points_value: 0, interval: 0, reset_daily: false, requires_approval: false, requires_photo: false }));
    expect(saved).toHaveBeenCalledOnce(); expect(close).toHaveBeenCalledOnce();
  });
  it('saves and clears the name while keeping the form open', async () => {
    const close = vi.fn(); const interaction = userEvent.setup();
    render(<QuickChore user={user} onClose={close} onSaved={vi.fn()} />);
    await interaction.type(screen.getByLabelText('Chore name'), 'Fix shelf');
    await interaction.click(screen.getByRole('button', { name: 'Save and add another' }));
    await waitFor(() => expect((screen.getByLabelText('Chore name') as HTMLInputElement).value).toBe(''));
    expect(close).not.toHaveBeenCalled();
    await interaction.type(screen.getByLabelText('Chore name'), 'Water plants');
    await interaction.click(screen.getByRole('button', { name: 'Save' }));
    expect(api.chores.quickAdd).toHaveBeenCalledTimes(2);
  });
  it('keeps the draft when saving fails', async () => {
    vi.mocked(api.chores.quickAdd).mockRejectedValue(new Error('Connection lost'));
    const close = vi.fn(); const interaction = userEvent.setup();
    render(<QuickChore user={user} onClose={close} onSaved={vi.fn()} />);
    await interaction.type(screen.getByLabelText('Chore name'), 'Fix shelf');
    await interaction.click(screen.getByRole('button', { name: 'Save' }));
    expect(await screen.findByRole('alert')).toHaveProperty('textContent', 'Connection lost');
    expect((screen.getByLabelText('Chore name') as HTMLInputElement).value).toBe('Fix shelf');
    expect(close).not.toHaveBeenCalled();
  });
  it('allows an explicit reset and repetition without changing the defaults', async () => {
    const interaction = userEvent.setup();
    render(<QuickChore user={user} onClose={vi.fn()} onSaved={vi.fn()} />);
    await interaction.type(screen.getByLabelText('Chore name'), 'Practice');
    await interaction.click(screen.getByText('More options'));
    await interaction.selectOptions(screen.getByLabelText('Repeat'), 'repeat');
    await interaction.selectOptions(screen.getByLabelText('Unfinished chores'), 'reset');
    await interaction.click(screen.getByRole('button', { name: 'Save' }));
    expect(api.chores.quickAdd).toHaveBeenCalledWith(expect.objectContaining({ reset_daily: true, interval: 1 }));
  });
});
