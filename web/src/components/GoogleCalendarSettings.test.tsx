// @vitest-environment jsdom
import { afterEach, beforeAll, beforeEach, expect, it, vi } from 'vitest';
import { cleanup, render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import i18n from '../i18n';
import { api } from '../api';
import { CalendarCreationToggle, GoogleCalendarSettings } from './GoogleCalendarSettings';

vi.mock('../api', () => ({ api: { googleCalendar: { status: vi.fn(), select: vi.fn(), disconnect: vi.fn(), connect: vi.fn() } } }));
beforeAll(async () => { await i18n.changeLanguage('en'); });
beforeEach(() => { vi.clearAllMocks(); vi.mocked(api.googleCalendar.status).mockResolvedValue({ configured: true, connected: true, calendar_id: 'primary', calendars: [{ id: 'personal', summary: 'Personal', primary: true }, { id: 'family', summary: 'Family', primary: false }] }); });
afterEach(cleanup);

it('saves the default calendar and disconnects the account from settings', async () => {
  const interaction = userEvent.setup();
  render(<GoogleCalendarSettings />);
  const select = await screen.findByLabelText('Default calendar');
  expect(select).toHaveProperty('value', 'personal');
  await interaction.selectOptions(select, 'family');
  expect(api.googleCalendar.select).toHaveBeenCalledWith('family');
  await interaction.click(screen.getByRole('button', { name: 'Disconnect' }));
  expect(api.googleCalendar.disconnect).toHaveBeenCalledOnce();
});

it('shows setup guidance and prevents enabling export without a connection', async () => {
  vi.mocked(api.googleCalendar.status).mockResolvedValue({ configured: false, connected: false, calendar_id: '', calendars: [] });
  render(<CalendarCreationToggle enabled={false} onChange={vi.fn()} />);
  expect(await screen.findByText('Google Calendar needs a one-time setup by the server owner before accounts can connect.')).toBeTruthy();
  expect(screen.getByRole('checkbox')).toHaveProperty('disabled', true);
});

it('allows enabling calendars for assignees even when the creator has no Google connection', async () => {
  const interaction = userEvent.setup(); const change = vi.fn();
  vi.mocked(api.googleCalendar.status).mockResolvedValue({ configured: true, connected: false, calendar_id: '', calendars: [] });
  render(<CalendarCreationToggle enabled={false} onChange={change} />);
  expect(await screen.findByText(/Adds this chore to each assigned person's chosen Google calendar/)).toBeTruthy();
  await interaction.click(screen.getByRole('checkbox'));
  expect(change).toHaveBeenCalledWith(true);
});
