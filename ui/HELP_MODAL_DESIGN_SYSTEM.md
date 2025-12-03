# Help Modal Design System

## Color Palette

### Primary Colors (Station Blue)
- **Primary Blue**: `#0084FF` - Use for active states, primary actions, icons
- **Primary Blue RGB**: `rgb(0, 132, 255)` or `bg-[#0084FF]`

### Neutral Grays
- **Text Primary**: `text-gray-900` - Main headings, important text
- **Text Secondary**: `text-gray-700` - Body text, descriptions
- **Text Tertiary**: `text-gray-600` - Supporting text, labels
- **Text Muted**: `text-gray-500` - Placeholder, disabled

### Backgrounds
- **Page Background**: `bg-[#F8FAFB]` - Light blue-gray (from design system)
- **Card Background**: `bg-white` - Pure white for cards
- **Subtle Background**: `bg-gray-50` - Very light gray for containers
- **Hover Background**: `bg-gray-100` - Interactive hover states

### Borders
- **Primary Border**: `border-gray-200` - Standard borders
- **Subtle Border**: `border-gray-100` - Very light dividers

## Typography

### Headings
```tsx
// Section Title (h3)
className="text-base font-semibold text-gray-900 mb-3"

// Subsection Title
className="text-sm font-medium text-gray-900 mb-2"

// Label/Caption
className="text-xs font-medium text-gray-700 uppercase tracking-wider"
```

### Body Text
```tsx
// Primary Body
className="text-sm text-gray-700 leading-relaxed"

// Secondary Body
className="text-xs text-gray-600"

// Code/Monospace
className="font-mono text-xs text-gray-800"
```

## Component Patterns

### Section Container
```tsx
<div id="section-id" className="space-y-6">
  <h3 className="text-base font-semibold text-gray-900 mb-3">
    Section Title
  </h3>
  <div className="bg-gray-50 border border-gray-200 rounded-lg p-4">
    {/* Content */}
  </div>
</div>
```

### Card Grid (2 columns)
```tsx
<div className="grid grid-cols-2 gap-3">
  <div className="bg-white border border-gray-200 rounded-lg p-3">
    <div className="font-medium text-gray-900 text-sm mb-1">
      Card Title
    </div>
    <div className="text-xs text-gray-600">
      Card description
    </div>
  </div>
</div>
```

### Info Box
```tsx
<div className="bg-[#F8FAFB] border border-gray-200 rounded-lg p-4">
  <div className="text-sm text-gray-700 leading-relaxed">
    Information content
  </div>
</div>
```

### Warning/Alert Box
```tsx
<div className="bg-amber-50 border border-amber-200 rounded-lg p-4">
  <div className="flex items-start gap-2">
    <AlertCircle className="h-4 w-4 text-amber-600 flex-shrink-0 mt-0.5" />
    <div className="text-xs text-amber-800">
      Warning message
    </div>
  </div>
</div>
```

## Icon Usage

### Icon Colors by Purpose
- **Primary Action**: `text-[#0084FF]` - Station blue
- **Neutral**: `text-gray-600` - Default icons
- **Success**: `text-emerald-600` - Positive states
- **Warning**: `text-amber-600` - Caution
- **Error**: `text-red-600` - Errors

### Icon Sizes
- **Small**: `h-4 w-4` - In-line with text, cards
- **Medium**: `h-5 w-5` - Section headers, prominent
- **Large**: `h-6 w-6` - Hero elements

### Icon with Text Pattern
```tsx
<div className="flex items-center gap-2">
  <Icon className="h-5 w-5 text-[#0084FF]" />
  <span className="text-sm font-medium text-gray-900">Label</span>
</div>
```

## Spacing System

### Section Spacing
- Between sections: `space-y-6` or `mb-6`
- Within sections: `space-y-3` or `mb-3`
- Tight spacing: `space-y-2` or `mb-2`

### Padding
- Cards: `p-4`
- Large containers: `p-6`
- Compact cards: `p-3`

## Anti-Patterns (Don't Do This)

❌ **Random Colors**
```tsx
// BAD: Random blues, purples, cyans
<Icon className="text-blue-600" />
<Icon className="text-purple-600" />
<Icon className="text-cyan-600" />
```

✅ **Consistent Colors**
```tsx
// GOOD: Station blue or neutral grays
<Icon className="text-[#0084FF]" />
<Icon className="text-gray-600" />
```

❌ **Inconsistent Backgrounds**
```tsx
// BAD: blue-50, purple-50, cyan-50 mixed
<div className="bg-blue-50">
<div className="bg-purple-50">
```

✅ **Consistent Backgrounds**
```tsx
// GOOD: gray-50 or F8FAFB from design system
<div className="bg-gray-50">
<div className="bg-[#F8FAFB]">
```

❌ **Arbitrary Icon Colors**
```tsx
// BAD: Different color for each icon
<Clock className="text-blue-600" />
<Zap className="text-purple-600" />
<DollarSign className="text-orange-600" />
```

✅ **Purposeful Icon Colors**
```tsx
// GOOD: Neutral or primary blue
<Clock className="text-gray-600" />
<Zap className="text-gray-600" />
<DollarSign className="text-gray-600" />
// OR all primary blue for emphasis
<Clock className="text-[#0084FF]" />
```

## Example: Well-Designed Section

```tsx
<div id="example-section" className="space-y-6">
  <h3 className="text-base font-semibold text-gray-900 mb-3">
    Section Title
  </h3>
  
  <div className="bg-gray-50 border border-gray-200 rounded-lg p-4">
    <div className="text-sm text-gray-700 leading-relaxed mb-4">
      Introduction paragraph explaining the concept.
    </div>
    
    <div className="grid grid-cols-2 gap-3">
      <div className="bg-white border border-gray-200 rounded-lg p-3">
        <div className="flex items-center gap-2 mb-2">
          <Icon className="h-4 w-4 text-[#0084FF]" />
          <div className="font-medium text-gray-900 text-sm">Feature Name</div>
        </div>
        <div className="text-xs text-gray-600">
          Description of the feature and its purpose.
        </div>
      </div>
    </div>
  </div>
</div>
```

