import { test, expect } from '@playwright/test'

// Offer Comparison — light e2e (feature plan §5.6). The load-time auto-run
// uses the KZT/EUR example and may depend on external rates, so it is never
// load-bearing here: we assert the PRE-FILL (UI behavior), then run our own
// deterministic USD/USD + known-cities comparison (embedded COL data only).
test.describe('offer comparison', () => {
  test('pre-filled form; deterministic compare; verdict updates on re-run', async ({ page }) => {
    await page.goto('/compare')

    // The example is pre-filled before any typing (payoff-before-effort).
    const amounts = page.getByPlaceholder('650 000')
    await expect(amounts.first()).toHaveValue('800 000')
    await expect(amounts.nth(1)).toHaveValue('3 500')

    // Deterministic comparison: same currency, both cities in the dataset.
    await amounts.first().fill('3400')
    await page.locator('.compare-card select').first().selectOption('USD')
    await page.locator('.compare-card input[list="col-cities"]').first().fill('Almaty')
    await amounts.nth(1).fill('5500')
    await page.locator('.compare-card select').nth(1).selectOption('USD')
    await page.locator('.compare-card input[list="col-cities"]').nth(1).fill('Lisbon')
    await page.getByRole('button', { name: 'Compare', exact: true }).click()

    // COL 34 vs 55 with equal real value (3400/.34 == 5500/.55 == 10000):
    // +62% by FX, "about the same" in real terms — a great honesty demo.
    const verdict = page.locator('.compare-verdict')
    await expect(verdict).toContainText('about 62% more by exchange rate')
    await expect(verdict).toContainText('about the same in real purchasing power')

    // Bars for both groups render.
    await expect(page.locator('.recharts-surface')).toBeVisible()

    // Change one amount, re-run: the verdict must update.
    await amounts.nth(1).fill('8000')
    await page.getByRole('button', { name: 'Compare', exact: true }).click()
    await expect(verdict).toContainText('about 135% more by exchange rate')

    // Share button is present (download itself isn't asserted in e2e).
    await expect(page.getByRole('button', { name: 'Share this result' })).toBeVisible()

    // Notes include the dataset disclaimer with its update date.
    await expect(page.locator('.compare-notes')).toContainText('approximate')
  })
})
