# Design System

## Intent

The interface is used in short weekday gaps under ordinary office or home
lighting. It should feel quiet, precise, and easy to scan, leaving the detailed
reading experience to Telegram.

`theme.md` was empty when this system was initialized, so this is a temporary,
centralized baseline. Replace these tokens when the intended reference is added
without changing the product information architecture.

## Color

Use a restrained light strategy with literal white as the main background. Burnt
orange is reserved for primary actions and current selection. Blue is a
secondary informational accent. All authored colors use OKLCH.

- Background: `oklch(1 0 0)`
- Surface: `oklch(0.975 0 0)`
- Ink: `oklch(0.19 0.014 48)`
- Muted ink: `oklch(0.46 0.018 48)`
- Primary: `oklch(0.45 0.13 48)`
- Accent: `oklch(0.48 0.12 248)`
- Success: `oklch(0.43 0.1 150)`
- Warning surface: `oklch(0.95 0.035 80)`
- Error: `oklch(0.48 0.16 28)`

## Typography

Use the native system sans-serif stack for every product surface. Headings are
compact and moderately weighted; labels and body copy use familiar sentence
case. Long explanatory copy is limited to 70 characters per line.

## Layout

Desktop uses a narrow persistent navigation rail and a single content canvas.
Mobile replaces the rail with a compact top identity and bottom navigation.
Content uses a maximum width of 1,120 pixels. Related controls may share a
bordered panel, but nested cards are not used.

## Components

- Buttons use one 10-pixel radius, clear hover/focus/disabled states, and no
  decorative shadow.
- Panels use a subtle neutral border or a contrasting surface, never both a wide
  shadow and border.
- Status pills use semantic color and short plain-language labels.
- Empty states explain what is missing, why it matters, and the next available
  action.
- Form controls use native semantics, visible labels, and 44-pixel minimum touch
  targets.

## Motion

Transitions are limited to 160-200 milliseconds and communicate hover, focus, or
navigation state. Reduced-motion preferences remove nonessential movement.
