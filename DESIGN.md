# Design System

## Overview
MyCloudDrive uses a light, task-focused product UI. The interface should feel approachable and organized, with restrained color, soft structure, and clear state transitions. The system supports file operations, storage configuration, workspace switching, and knowledge base management in one continuous shell.

## Color

### Strategy
Restrained. Tinted neutrals carry most surfaces, with a single cool accent for active state and primary action, plus semantic colors for success, warning, and error.

### Tokens
- `--mcd-bg`: app background, warm-cool neutral
- `--mcd-surface`: primary content surface
- `--mcd-surface-alt`: sidebar / toolbar / grouped region surface
- `--mcd-border`: default border
- `--mcd-border-strong`: emphasized border
- `--mcd-text`: primary text
- `--mcd-text-muted`: secondary text
- `--mcd-accent`: primary action and selected state
- `--mcd-accent-soft`: accent-tinted background
- `--mcd-success`, `--mcd-warning`, `--mcd-danger`: semantic states

### Guidance
- Use OKLCH values where custom tokens are defined.
- Avoid pure black and pure white.
- Reserve accent usage for selected navigation, primary CTA, and focused state.

## Typography
- Primary family: system sans with Chinese fallback, optimized for product readability.
- One family across headings, labels, forms, and data.
- Tight product scale with stronger weight changes than size jumps.
- Headings should feel calm and competent, not promotional.

## Layout
- Three-zone shell: top context bar, left navigation, main work area.
- Side navigation uses grouped sections with clear labels and subtle nesting.
- Main pages should start with a summary band or page header before dense controls.
- Tables remain standard Ant Design tables, but are visually integrated with the surface system.

## Components
- Navigation items: rounded, low-contrast default, accent-tinted selected state.
- Panels: soft border, moderate radius, light shadow only where grouping needs help.
- Buttons: primary action is filled accent, secondary action is tonal or outlined, destructive action remains explicit.
- Empty states: concise explanation plus a next step.
- Status chips: semantic tint plus text, never color alone.

## Motion
- Use short ease-out transitions for hover, focus, and panel reveal.
- No decorative looping animations in the authenticated product shell.
- Keep motion under 200ms for standard interactions.

## Voice
- Labels should be direct and operational.
- Helper copy should tell users what to do next.
- Avoid banner-style marketing language inside the app shell.
