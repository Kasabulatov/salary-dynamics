import { test, expect } from '@playwright/test'
import { uniqueEmail, PASSWORD, register } from './helpers.js'

test.describe('authentication', () => {
  test('landing page greets anonymous visitors', async ({ page }) => {
    await page.goto('/')
    await expect(page.getByRole('heading', { name: 'Your salary. In real terms.' })).toBeVisible()
    await expect(page.getByRole('link', { name: 'Try it — no account needed' })).toBeVisible()
  })

  test('register -> logout -> login round trip', async ({ page }) => {
    const email = uniqueEmail('auth')
    await register(page, email)

    // Logged in: nav shows the account email.
    await expect(page.locator('.nav-user')).toHaveText(email)

    await page.getByRole('button', { name: 'Sign out' }).click()
    await expect(page.getByRole('heading', { name: 'Your salary. In real terms.' })).toBeVisible()

    await page.goto('/login')
    await page.getByLabel('Email').fill(email)
    await page.getByLabel('Password').fill(PASSWORD)
    await page.getByRole('button', { name: 'Sign in', exact: true }).click()
    await expect(page.getByRole('heading', { name: 'Salary Dynamics.' })).toBeVisible()
  })

  test('rejects wrong password', async ({ page }) => {
    const email = uniqueEmail('badpw')
    await register(page, email)
    await page.getByRole('button', { name: 'Sign out' }).click()

    await page.goto('/login')
    await page.getByLabel('Email').fill(email)
    await page.getByLabel('Password').fill('definitely-wrong-1')
    await page.getByRole('button', { name: 'Sign in', exact: true }).click()
    await expect(page.locator('.form-error')).toContainText('invalid email or password')
  })

  test('duplicate registration is rejected', async ({ page }) => {
    const email = uniqueEmail('dup')
    await register(page, email)
    await page.getByRole('button', { name: 'Sign out' }).click()

    await page.goto('/register')
    await page.getByLabel('Email').fill(email)
    await page.getByLabel('Password (min. 8 characters)').fill(PASSWORD)
    await page.getByLabel('Confirm password').fill(PASSWORD)
    await page.getByRole('button', { name: 'Register' }).click()
    await expect(page.locator('.form-error')).toContainText('already registered')
  })
})
