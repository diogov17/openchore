import { test, expect } from '@playwright/test';
import { apiGet, authHeaders, localDateStr, loginAsAdmin, selectUser } from './helpers/setup';

test.describe('Home points and chore reminders', () => {
  test('Manage Today includes Home chores on the assigned person card', async ({ page }) => {
    const title = `E2E Manage Today Home ${Date.now()}`;
    const headers = await authHeaders(1);
    const create = await page.request.post('/api/chores', {
      headers,
      data: { title, category: 'home', points_value: 4 },
    });
    expect(create.ok()).toBeTruthy();
    const chore = await create.json() as { id: number };
    const assign = await page.request.put(`/api/home/backlog/${chore.id}/assignment`, {
      headers,
      data: { user_id: 3 },
    });
    expect(assign.ok()).toBeTruthy();

    await loginAsAdmin(page);
    await page.getByRole('button', { name: /Emma/ }).click();
    await expect(page.getByText(title)).toBeVisible();
    await expect(page.getByText('Home', { exact: true }).last()).toBeVisible();

    const markDone = page.getByRole('button', { name: `Mark "${title}" done for Emma` });
    await markDone.click();
    const undo = page.getByRole('button', { name: `Undo "${title}" for Emma` });
    await expect(undo).toBeVisible();
    await undo.click();
    await expect(page.getByRole('button', { name: `Mark "${title}" done for Emma` })).toBeVisible();
  });

  test('a Home chore assigned to all admins is listed for both and can be claimed once', async ({ page }) => {
    const title = `E2E Shared Home ${Date.now()}`;
    const adminOne = await authHeaders(1);
    const create = await page.request.post('/api/chores', {
      headers: adminOne,
      data: { title, category: 'home', points_value: 8 },
    });
    expect(create.ok()).toBeTruthy();
    const chore = await create.json() as { id: number };

    await loginAsAdmin(page);
    await page.getByRole('button', { name: /^Chores$/i }).click();
    const assigneePicker = page.getByRole('button', { name: `Assign ${title}` });
    await expect(assigneePicker).toBeVisible();
    await assigneePicker.click();
    await page.getByRole('button', { name: 'All admins' }).click();
    await expect(page.getByText('Assigned to all admins')).toBeVisible();

    await page.getByRole('button', { name: /^Today$/i }).click();
    const alexCard = page.getByRole('button', { name: /Alex/ });
    await alexCard.click();
    await expect(page.getByText(title)).toBeVisible();
    await alexCard.click();
    await page.getByRole('button', { name: /Jamie/ }).click();
    await expect(page.getByText(title)).toBeVisible();

    const date = localDateStr();
    for (const adminId of [1, 2]) {
      const assigned = await apiGet(page, `/home/assigned?date=${date}`, adminId) as Array<{ chore_id: number; assigned_to_all?: boolean }>;
      expect(assigned).toContainEqual(expect.objectContaining({ chore_id: chore.id, assigned_to_all: true }));
    }

    const firstCompletion = await page.request.post(`/api/home/assigned/${chore.id}/complete`, {
      headers: adminOne,
      data: { completion_date: date },
    });
    expect(firstCompletion.status()).toBe(201);

    const secondCompletion = await page.request.post(`/api/home/assigned/${chore.id}/complete`, {
      headers: await authHeaders(2),
      data: { completion_date: date },
    });
    expect(secondCompletion.status()).toBe(409);
  });

  test('an assigned Home chore shows its configured points on Today', async ({ page }) => {
    const title = `E2E Home Points ${Date.now()}`;
    const headers = await authHeaders(1);
    const create = await page.request.post('/api/chores', {
      headers,
      data: { title, category: 'home', points_value: 5 },
    });
    expect(create.ok()).toBeTruthy();
    const chore = await create.json() as { id: number };

    const assign = await page.request.put(`/api/home/backlog/${chore.id}/assignment`, {
      headers,
      data: { user_id: 3 }, // Emma in the seeded E2E household.
    });
    expect(assign.ok()).toBeTruthy();

    await selectUser(page, 'Emma');
    const row = page.getByText(title).locator('..').locator('..');
    await expect(row).toContainText('5 pts');
    await expect(row).not.toContainText('0 pts');
  });

  test('a reminder selected during chore creation is saved and shown on Today', async ({ page }) => {
    const title = `E2E Reminder ${Date.now()}`;
    await loginAsAdmin(page);
    await page.getByRole('button', { name: /^Chores$/i }).click();
    await page.getByRole('button', { name: /Add Chore/i }).click();
    await page.getByPlaceholder('e.g. Empty the dishwasher').fill(title);
    await page.getByRole('button', { name: /Next/i }).click();

    await page.getByRole('button', { name: 'Emma' }).click();
    await page.getByRole('button', { name: 'Every day' }).click();
    await page.locator('input[type="time"]').nth(2).fill('08:45');
    await page.getByRole('button', { name: /Next/i }).click();
    await expect(page.getByText('08:45')).toBeVisible();
    await page.getByRole('button', { name: /Create Chore/i }).click();
    await expect(page.getByText('New Chore')).not.toBeVisible({ timeout: 10_000 });

    const choresResponse = await page.request.get('/api/chores', { headers: await authHeaders(1) });
    expect(choresResponse.ok()).toBeTruthy();
    const chores = await choresResponse.json() as Array<{ id: number; title: string }>;
    const chore = chores.find(item => item.title === title);
    expect(chore).toBeTruthy();

    const schedulesResponse = await page.request.get(`/api/chores/${chore!.id}/schedules`, { headers: await authHeaders(1) });
    expect(schedulesResponse.ok()).toBeTruthy();
    const schedules = await schedulesResponse.json() as Array<{ reminder_at?: string }>;
    expect(schedules.length).toBeGreaterThan(0);
    expect(schedules.every(schedule => schedule.reminder_at === '08:45')).toBeTruthy();

    await selectUser(page, 'Emma');
    await expect(page.getByText(title)).toBeVisible();
    await expect(page.getByText('Reminder 8:45 AM')).toBeVisible();
  });
});
