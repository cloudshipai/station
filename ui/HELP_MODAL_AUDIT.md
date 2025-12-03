# Help Modal Content Audit

## Issues Found

### 1. **SwimlanePage.tsx** (Agent Runs)
**Problems:**
- ❌ Random colored icon backgrounds: `bg-blue-100`, `bg-green-100`, `bg-purple-100`, `bg-orange-100`
- ❌ Random icon colors: `text-blue-600`, `text-green-600`, `text-purple-600`, `text-orange-600`
- ❌ Inconsistent with Station blue theme

**Fix:**
- Use neutral gray backgrounds: `bg-gray-100`
- Use Station blue for icons: `text-[#0084FF]` or keep neutral `text-gray-600`

### 2. **ReportsPage.tsx** (Reports & Evaluation)
**Problems:**
- ❌ Random colored step numbers: `bg-blue-600`, `bg-purple-600`, `bg-green-600`, `bg-orange-600`
- ❌ Random icon colors: `text-blue-600` scattered throughout
- ❌ Not using Station blue consistently

**Fix:**
- Use Station blue for all step numbers: `bg-[#0084FF]`
- Use Station blue or neutral gray for icons consistently

### 3. **MCPDirectoryPage.tsx** (MCP Directory)
**Need to check:**
- Icon colors
- Background colors
- Consistency with design system

### 4. **App.tsx** (Bundles)
**Problems (from search results):**
- ❌ Random icon colors: `text-purple-600`, `text-blue-600`, `text-cyan-600`
- ❌ Should use Station blue or neutral

### 5. **FakerBuilderModal.tsx** (AI Faker)
**Problems:**
- ❌ `text-purple-600` for Sparkles icon
- ❌ Should use Station blue

### 6. **AgentsLayout.tsx**
**Need to check:**
- Icon colors
- Background colors

## Design System Rules

### Icon Colors (Choose One Strategy)

**Option A: All Neutral**
```tsx
<Icon className="h-5 w-5 text-gray-600" />
```

**Option B: All Station Blue**
```tsx
<Icon className="h-5 w-5 text-[#0084FF]" />
```

**Option C: Semantic (Recommended)**
```tsx
// Primary/important: Station blue
<Icon className="h-5 w-5 text-[#0084FF]" />

// Secondary/neutral: Gray
<Icon className="h-5 w-5 text-gray-600" />

// Success states only: Emerald
<CheckCircle className="h-5 w-5 text-emerald-600" />

// Warning states only: Amber
<AlertCircle className="h-5 w-5 text-amber-600" />

// Error states only: Red
<XCircle className="h-5 w-5 text-red-600" />
```

### Icon Backgrounds
**Never use colored backgrounds for decoration**
```tsx
// ❌ BAD
<div className="p-2 rounded-lg bg-blue-100">
  <Icon className="h-5 w-5 text-blue-600" />
</div>

// ✅ GOOD - Neutral background
<div className="p-2 rounded-lg bg-gray-100">
  <Icon className="h-5 w-5 text-[#0084FF]" />
</div>

// ✅ BETTER - No background, just icon
<Icon className="h-5 w-5 text-[#0084FF]" />
```

### Step Numbers
**Use Station blue consistently**
```tsx
// ❌ BAD - Rainbow colors
<div className="bg-blue-600">1</div>
<div className="bg-purple-600">2</div>
<div className="bg-green-600">3</div>

// ✅ GOOD - Consistent Station blue
<div className="bg-[#0084FF]">1</div>
<div className="bg-[#0084FF]">2</div>
<div className="bg-[#0084FF]">3</div>
```

## Fixing Priority

1. **High Priority** - Runs, Reports (user-facing, high traffic)
2. **Medium Priority** - MCP Directory, Bundles
3. **Low Priority** - Faker, Agents (less frequent)

## Implementation Plan

1. Create reusable components:
   - `<StepNumber>` - Consistent step numbering
   - `<FeatureCard>` - Consistent feature cards
   - `<InfoBox>` - Consistent info containers

2. Replace all instances of random colors with design system colors

3. Test visual consistency across all modals

