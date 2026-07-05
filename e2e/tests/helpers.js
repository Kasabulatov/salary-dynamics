// Shared E2E helpers. Every run registers fresh users (unique emails), so
// tests are independent of existing data and repeatable on a dirty DB.

let seq = 0
export function uniqueEmail(tag) {
  seq += 1
  return `e2e-${tag}-${Date.now()}-${seq}@example.com`
}

export const PASSWORD = 'e2e-password-123'

export async function register(page, email) {
  await page.goto('/register')
  await page.getByLabel('Email').fill(email)
  await page.getByLabel('Password (min. 8 characters)').fill(PASSWORD)
  await page.getByLabel('Confirm password').fill(PASSWORD)
  await page.getByRole('button', { name: 'Register' }).click()
  // Registration logs in and lands on the tracker.
  await page.getByRole('heading', { name: 'Salary Dynamics.' }).waitFor()
}

export async function addEntry(page, { amount, date, note }) {
  await page.getByPlaceholder('650 000').fill(amount)
  // USD keeps E2E independent of external rate APIs (same-currency path).
  await page.getByLabel('Salary Currency').selectOption('USD')
  await page.getByPlaceholder('2024-06-01 or 01.06.2024').fill(date)
  if (note) await page.getByPlaceholder('e.g. promotion, new job').fill(note)
  await page.getByRole('button', { name: 'Add', exact: true }).click()
}
