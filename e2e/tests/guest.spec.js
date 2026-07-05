import { test, expect } from '@playwright/test'
import { uniqueEmail, PASSWORD } from './helpers.js'

test.describe('guest mode', () => {
  test('guest adds an entry without an account; import on register', async ({ page }) => {
    await page.goto('/')
    await page.getByRole('link', { name: 'Try it — no account needed' }).click()

    // Guest banner explains the data lifetime.
    await expect(page.locator('.guest-banner')).toContainText('Guest mode')

    // Add a guest entry (sessionStorage only).
    await page.getByPlaceholder('650 000').fill('3000')
    await page.getByLabel('Salary Currency').selectOption('USD')
    await page.getByPlaceholder('2024-06-01 or 01.06.2024').fill('2024-02-01')
    await page.getByRole('button', { name: 'Add', exact: true }).click()
    await expect(page.locator('.entries-table tbody tr')).toHaveCount(1)

    // The stateless compute fills the converted column (USD -> same value).
    await expect(page.locator('.entries-table tbody tr').first())
      .toContainText('3,000', { timeout: 30_000 })

    // Register from guest mode: the form announces the import.
    await page.getByRole('link', { name: 'Create account to keep them' }).click()
    await expect(page.locator('.guest-import-note')).toContainText('1 guest entry')

    const email = uniqueEmail('guest')
    await page.getByLabel('Email').fill(email)
    await page.getByLabel('Password (min. 8 characters)').fill(PASSWORD)
    await page.getByLabel('Confirm password').fill(PASSWORD)
    await page.getByRole('button', { name: 'Register' }).click()

    // The guest entry survived into the account.
    await expect(page.getByRole('heading', { name: 'Salary Dynamics.' })).toBeVisible()
    await expect(page.locator('.entries-table tbody tr')).toHaveCount(1)
    await expect(page.locator('.entries-table tbody tr').first()).toContainText('2024-02-01')
  })
})
