// @vitest-environment jsdom
import { afterEach, beforeAll, expect, it, vi } from 'vitest';
import { cleanup, render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import i18n from '../../i18n';
import type { ScheduledChore, User } from '../../types';
import type { KidData } from './useKidData';
import { WeekScreen } from './WeekScreen';

beforeAll(async () => { await i18n.changeLanguage('en'); });
afterEach(cleanup);

it('checks off an earlier day with that day’s prerequisites and original date', async () => {
  const chore: ScheduledChore = { schedule_id: 42, chore_id: 8, title: 'Read', description: '', category: 'bonus', points_value: 5, missed_penalty_value: 0, requires_approval: false, requires_photo: false, assignment_type: 'individual', expiry_penalty: 'block', expiry_penalty_value: 0, available: true, expired: false, completed: false, date: '2026-10-21' };
  const data = { today: '2026-10-22', weekChores: [chore], togglingIds: new Set(), streak: null } as unknown as KidData;
  const user = { id: 7, name: 'Alex', paused: false } as User;
  const toggle = vi.fn(); const interaction = userEvent.setup();
  render(<WeekScreen user={user} data={data} topBar={null} viewOf={() => ({ cat: 'bonus', state: 'locked', canToggle: false, urgent: false, photo: false, retry: false, needsPhotoProof: false })} tts={false} onToggle={toggle} onPhoto={vi.fn()} onSpeak={vi.fn()} />);
  await interaction.click(screen.getByRole('button', { name: /Wednesday/ }));
  await interaction.click(screen.getByRole('button', { name: 'Mark complete' }));
  expect(toggle).toHaveBeenCalledWith(expect.objectContaining({ date: '2026-10-21', schedule_id: 42 }), expect.objectContaining({ canToggle: true }));
});
