import { test, expect } from '@playwright/test';

test.describe('Agents Page', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/agents');
  });

  test('should display agents page', async ({ page }) => {
    await expect(page.locator('text=/Agents|No agents/i').first()).toBeVisible({ timeout: 10000 });
  });

  test('should show agents list or empty state', async ({ page }) => {
    const content = page.locator('text=/agent|No agents found/i').first();
    await expect(content).toBeVisible({ timeout: 10000 });
  });

  test('should have navigation sidebar with agents link', async ({ page }) => {
    const agentsNavLink = page.locator('nav button, nav a').filter({ hasText: /Agents/i });
    await expect(agentsNavLink).toBeVisible();
  });

  test('should navigate to agents page from sidebar', async ({ page }) => {
    await page.goto('/');
    const agentsLink = page.locator('button, a').filter({ hasText: /Agents/i }).first();
    await agentsLink.click();
    await expect(page).toHaveURL(/\/agents/);
  });
});

test.describe('Agents Page - Environment Selection', () => {
  test('should have environment selector if environments exist', async ({ page }) => {
    await page.goto('/agents');
    
    const envSelector = page.locator('select').first();
    const selectorCount = await envSelector.count();
    
    if (selectorCount > 0) {
      await expect(envSelector).toBeVisible();
    }
  });
});

test.describe('Navigation', () => {
  test('should navigate between main sections', async ({ page }) => {
    await page.goto('/agents');

    await page.locator('button, a').filter({ hasText: /Workflows/i }).first().click();
    await expect(page).toHaveURL(/\/workflows/);

    await page.locator('button, a').filter({ hasText: /Runs/i }).first().click();
    await expect(page).toHaveURL(/\/runs/);

    await page.locator('button, a').filter({ hasText: /Agents/i }).first().click();
    await expect(page).toHaveURL(/\/agents/);
  });

  test('should display sidebar navigation items', async ({ page }) => {
    await page.goto('/');

    const navItems = ['Agents', 'Runs', 'Workflows', 'MCP Servers', 'Environments'];
    
    for (const item of navItems) {
      const navButton = page.locator('nav button, nav a').filter({ hasText: new RegExp(item, 'i') });
      await expect(navButton).toBeVisible();
    }
  });
});

test.describe('Runs Page', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/runs');
  });

  test('should display runs page', async ({ page }) => {
    await expect(page.locator('text=/Runs|No runs|Agent Runs/i').first()).toBeVisible({ timeout: 10000 });
  });

  test('should have status filter', async ({ page }) => {
    const statusFilter = page.locator('select').first();
    const filterCount = await statusFilter.count();
    
    if (filterCount > 0) {
      await expect(statusFilter).toBeVisible();
    }
  });
});
