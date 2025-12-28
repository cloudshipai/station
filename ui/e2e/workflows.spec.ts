import { test, expect } from '@playwright/test';

/**
 * Workflows Page E2E Tests
 * 
 * Prerequisites:
 * 1. Station backend running: go run cmd/main/main.go serve
 * 2. Station UI running: npm run dev (handled by playwright.config.ts webServer)
 * 
 * Run tests:
 *   npx playwright test workflows.spec.ts
 *   npm run test:e2e -- workflows.spec.ts
 */

test.describe('Workflows Page', () => {
  test.beforeEach(async ({ page }) => {
    // Navigate to workflows page
    await page.goto('/workflows');
  });

  test('should display workflows page with title', async ({ page }) => {
    // Check page title/heading
    await expect(page.getByRole('heading', { name: 'Workflows' })).toBeVisible();
  });

  test('should have definitions and runs tabs', async ({ page }) => {
    // Check for tab buttons
    const definitionsTab = page.getByRole('button', { name: /Definitions/i });
    const runsTab = page.getByRole('button', { name: /Runs/i });

    await expect(definitionsTab).toBeVisible();
    await expect(runsTab).toBeVisible();
  });

  test('should show empty state when no workflows exist', async ({ page }) => {
    // Check for empty state message (may show "No workflows defined" or workflow list)
    const emptyStateOrList = page.locator('text=/No workflows defined|workflow/i').first();
    await expect(emptyStateOrList).toBeVisible({ timeout: 10000 });
  });

  test('should switch between definitions and runs tabs', async ({ page }) => {
    // Click Runs tab
    await page.getByRole('button', { name: /Runs/i }).click();
    
    // Verify runs tab is active (should show run-related content or empty state)
    const runsContent = page.locator('text=/No workflow runs|runs|Run/i').first();
    await expect(runsContent).toBeVisible({ timeout: 5000 });

    // Click back to Definitions tab
    await page.getByRole('button', { name: /Definitions/i }).click();
    
    // Verify definitions content is visible
    await expect(page.getByRole('button', { name: /Definitions/i })).toBeVisible();
  });

  test('should have refresh button', async ({ page }) => {
    // Look for refresh button (icon button)
    const refreshButton = page.locator('button[title="Refresh"]');
    await expect(refreshButton).toBeVisible();
  });
});

test.describe('Workflow Runs Tab', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/workflows');
    // Switch to runs tab
    await page.getByRole('button', { name: /Runs/i }).click();
  });

  test('should display status filter dropdown', async ({ page }) => {
    // Check for status filter
    const statusFilter = page.locator('select').filter({ hasText: /All Statuses/i });
    await expect(statusFilter).toBeVisible();
  });

  test('should have workflow filter dropdown', async ({ page }) => {
    // Check for workflow filter
    const workflowFilter = page.locator('select').filter({ hasText: /All Workflows/i });
    await expect(workflowFilter).toBeVisible();
  });

  test('should show run count', async ({ page }) => {
    // Check for runs count display
    const runsCount = page.locator('text=/\\d+ runs/');
    await expect(runsCount).toBeVisible();
  });
});

test.describe('Workflow Detail Page', () => {
  test('should navigate to workflow detail when clicking a workflow', async ({ page }) => {
    await page.goto('/workflows');
    
    // This test will only work if there are workflows
    // Check if there are workflow items (not empty state)
    const workflowItems = page.locator('[class*="cursor-pointer"]').filter({ 
      has: page.locator('[class*="GitBranch"], svg') 
    });
    
    const count = await workflowItems.count();
    
    if (count > 0) {
      // Click the first workflow
      await workflowItems.first().click();
      
      // Should navigate to workflow detail page
      await expect(page).toHaveURL(/\/workflows\/[\w-]+/);
    } else {
      // Skip if no workflows - just verify empty state is shown
      await expect(page.locator('text=/No workflows defined/i')).toBeVisible();
    }
  });
});
