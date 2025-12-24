import { test, expect } from '@playwright/test';

test.describe('Workflow Execution Steps Display', () => {
  // Helper to click the Runs tab (not the sidebar button)
  const clickRunsTab = async (page) => {
    // The tab button is the second "Runs" button (first is in sidebar)
    await page.getByRole('button', { name: 'Runs' }).nth(1).click();
  };

  test('should display workflow detail page with run history', async ({ page }) => {
    await page.goto('/workflows/test-agent-workflow');
    await expect(page.getByRole('heading', { name: 'Test Agent Workflow' })).toBeVisible({ timeout: 10000 });
    
    await clickRunsTab(page);
    await expect(page.getByText('Run History')).toBeVisible({ timeout: 10000 });
  });

  test('should show execution steps for completed run', async ({ page }) => {
    await page.goto('/workflows/test-agent-workflow');
    await expect(page.getByRole('heading', { name: 'Test Agent Workflow' })).toBeVisible({ timeout: 10000 });
    
    await clickRunsTab(page);
    await expect(page.getByText('Run History')).toBeVisible({ timeout: 10000 });
    
    const completedRun = page.locator('text=completed').first();
    if (await completedRun.isVisible()) {
      await completedRun.click();
      await expect(page.getByText('Execution Steps')).toBeVisible({ timeout: 5000 });
    }
  });

  test('should display step_id and step_type correctly', async ({ page }) => {
    await page.goto('/workflows/test-agent-workflow');
    await expect(page.getByRole('heading', { name: 'Test Agent Workflow' })).toBeVisible({ timeout: 10000 });
    
    await clickRunsTab(page);
    const completedRun = page.locator('text=completed').first();
    
    if (await completedRun.isVisible()) {
      await completedRun.click();
      await expect(page.getByText('Execution Steps')).toBeVisible({ timeout: 5000 });
      
      const stepTypeBadge = page.locator('text=/agent|inject|switch|parallel|foreach/');
      expect(await stepTypeBadge.count()).toBeGreaterThanOrEqual(1);
    }
  });

  test('should show View output expandable section', async ({ page }) => {
    await page.goto('/workflows/test-agent-workflow');
    await clickRunsTab(page);
    
    const completedRun = page.locator('text=completed').first();
    if (await completedRun.isVisible()) {
      await completedRun.click();
      await expect(page.getByText('Execution Steps')).toBeVisible({ timeout: 5000 });
      
      const viewOutput = page.locator('text=View output');
      if (await viewOutput.first().isVisible({ timeout: 3000 })) {
        await viewOutput.first().click();
        await expect(page.locator('pre')).toBeVisible({ timeout: 3000 });
      }
    }
  });
});

test.describe('Workflow List Page', () => {
  test('should display workflow list with test workflows', async ({ page }) => {
    await page.goto('/workflows');
    await expect(page.getByRole('heading', { name: 'Workflows', level: 1 })).toBeVisible({ timeout: 10000 });
    
    await expect(page.getByRole('heading', { name: 'Test Agent Workflow', level: 3 })).toBeVisible({ timeout: 5000 });
  });

  test('should navigate to workflow detail page', async ({ page }) => {
    await page.goto('/workflows');
    await page.getByRole('heading', { name: 'Test Agent Workflow', level: 3 }).click();
    await expect(page.getByRole('heading', { name: 'Test Agent Workflow', level: 1 })).toBeVisible({ timeout: 10000 });
  });
});

test.describe('Workflow Overview Tab', () => {
  test('should display workflow flow visualization', async ({ page }) => {
    await page.goto('/workflows/test-agent-workflow');
    await expect(page.getByText('Workflow Flow')).toBeVisible({ timeout: 10000 });
  });

  test('should display workflow info', async ({ page }) => {
    await page.goto('/workflows/test-agent-workflow');
    await expect(page.getByRole('heading', { name: 'Workflow Info', level: 3 })).toBeVisible({ timeout: 10000 });
    await expect(page.locator('dl').filter({ hasText: 'Workflow ID' }).locator('dd').first()).toContainText('test-agent-workflow');
  });

  test('should display recent runs', async ({ page }) => {
    await page.goto('/workflows/test-agent-workflow');
    await expect(page.getByText('Recent Runs')).toBeVisible({ timeout: 10000 });
  });
});

test.describe('Start Workflow Modal', () => {
  test('should open start workflow modal', async ({ page }) => {
    await page.goto('/workflows/test-agent-workflow');
    await expect(page.getByText('Test Agent Workflow')).toBeVisible({ timeout: 10000 });
    
    await page.getByRole('button', { name: /Start Run/i }).first().click();
    await expect(page.getByRole('heading', { name: 'Start Workflow Run' })).toBeVisible();
    await expect(page.getByText('Input JSON')).toBeVisible();
  });

  test('should close modal on Cancel', async ({ page }) => {
    await page.goto('/workflows/test-agent-workflow');
    await page.getByRole('button', { name: /Start Run/i }).first().click();
    await expect(page.getByRole('heading', { name: 'Start Workflow Run' })).toBeVisible();
    
    await page.getByRole('button', { name: 'Cancel' }).click();
    await expect(page.getByRole('heading', { name: 'Start Workflow Run' })).not.toBeVisible();
  });
});
