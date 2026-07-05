import { test, expect } from '@playwright/test'
import { uniqueEmail, register, addEntry } from './helpers.js'

test.describe('salary tracker', () => {
  test('add, edit, delete an entry; chart renders', async ({ page }) => {
    await register(page, uniqueEmail('crud'))

    // Add (USD -> conversion equals the amount, no external APIs).
    await addEntry(page, { amount: '5000', date: '2024-01-15', note: 'first job' })
    const row = page.locator('.entries-table tbody tr')
    await expect(row).toHaveCount(1)
    await expect(row.first()).toContainText('2024-01-15')
    await expect(row.first()).toContainText('5,000 USD')
    await expect(row.first()).toContainText('first job')

    // Amount input formats with thousands separators as you type.
    await expect(page.getByPlaceholder('650 000')).toHaveValue('')

    // Hero stat appears once the daily series loads.
    await expect(page.locator('.hero-value')).toContainText('5,000', { timeout: 30_000 })

    // The chart SVG draws.
    await expect(page.locator('.recharts-surface').first()).toBeVisible()

    // Edit: change the amount.
    await page.getByRole('button', { name: 'Edit' }).first().click()
    await page.getByPlaceholder('650 000').fill('6000')
    await page.getByRole('button', { name: 'Save' }).click()
    await expect(row.first()).toContainText('6,000 USD')

    // Delete (accept the confirm dialog).
    page.once('dialog', (d) => d.accept())
    await page.getByRole('button', { name: 'Delete' }).first().click()
    await expect(page.locator('.entries-table tbody tr')).toHaveCount(0)
  })

  test('rejects invalid input inline', async ({ page }) => {
    await register(page, uniqueEmail('valid'))

    await addEntry(page, { amount: '0', date: '2024-01-15' })
    await expect(page.locator('.form-error')).toContainText('greater than 0')
  })

  test('date field accepts typed European format', async ({ page }) => {
    await register(page, uniqueEmail('date'))

    await page.getByPlaceholder('650 000').fill('4000')
    await page.getByLabel('Salary Currency').selectOption('USD')
    await page.getByPlaceholder('2024-06-01 or 01.06.2024').fill('15.03.2024')
    await page.getByRole('button', { name: 'Add', exact: true }).click()
    await expect(page.locator('.entries-table tbody tr').first()).toContainText('2024-03-15')
  })
})
