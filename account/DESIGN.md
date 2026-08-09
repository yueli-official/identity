---
name: Yueli Account
description: A restrained identity surface that makes public profiles and stable account references easy to verify.
colors:
  primary-teal: "var(--ui-primary)"
  page-canvas: "var(--yueli-surface-page)"
  card-surface: "var(--yueli-surface-card)"
  elevated-surface: "var(--ui-bg-elevated)"
  text-default: "var(--ui-text)"
  text-highlighted: "var(--ui-text-highlighted)"
  text-muted: "var(--ui-text-muted)"
  text-inverted: "var(--ui-text-inverted)"
  border-default: "var(--ui-border)"
typography:
  display:
    fontFamily: "Space Grotesk, DM Sans, system-ui, sans-serif"
    fontSize: "1.5rem"
    fontWeight: 600
    lineHeight: 1.2
  title:
    fontFamily: "Space Grotesk, DM Sans, system-ui, sans-serif"
    fontSize: "1.125rem"
    fontWeight: 600
    lineHeight: 1.5
  body:
    fontFamily: "DM Sans, system-ui, sans-serif"
    fontSize: "1rem"
    fontWeight: 400
    lineHeight: 1.75
  label:
    fontFamily: "DM Sans, system-ui, sans-serif"
    fontSize: "0.75rem"
    fontWeight: 500
    lineHeight: 1.5
  data:
    fontFamily: "ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, Liberation Mono, monospace"
    fontSize: "0.875rem"
    fontWeight: 600
    lineHeight: 1.5
rounded:
  base: "0.5rem"
  feature: "0.75rem"
  card: "1rem"
  full: "9999px"
spacing:
  xs: "0.5rem"
  sm: "0.75rem"
  md: "1rem"
  lg: "1.5rem"
  xl: "2rem"
components:
  button-primary:
    backgroundColor: "{colors.primary-teal}"
    textColor: "{colors.text-inverted}"
    rounded: "{rounded.base}"
    padding: "0.5rem 0.75rem"
  button-neutral-outline:
    backgroundColor: "{colors.card-surface}"
    textColor: "{colors.text-default}"
    rounded: "{rounded.base}"
    padding: "0.5rem 0.75rem"
  field-outline:
    backgroundColor: "{colors.card-surface}"
    textColor: "{colors.text-highlighted}"
    rounded: "{rounded.base}"
    padding: "0.375rem 0.625rem"
  card:
    backgroundColor: "{colors.card-surface}"
    textColor: "{colors.text-default}"
    rounded: "{rounded.card}"
    padding: "1rem"
  profile-card:
    backgroundColor: "{colors.card-surface}"
    textColor: "{colors.text-default}"
    rounded: "{rounded.card}"
    padding: "0 1.25rem 1.5rem"
  stable-key-panel:
    backgroundColor: "{colors.elevated-surface}"
    textColor: "{colors.text-highlighted}"
    rounded: "{rounded.feature}"
    padding: "0.75rem 1rem"
---

# Design System: Yueli Account

## Overview

**Creative North Star: "The Public Identity Credential"**

Yueli Account treats identity as calm, verifiable infrastructure. Its warm-neutral surfaces, precise teal accents, soft corners, and cover-over-avatar composition make the interface personal without turning it into a social dashboard. Information density stays moderate and the strongest emphasis belongs to names, security actions, and stable identity references.

The public profile is the clearest expression of this system: visitors confirm the display name and mutable Handle first, read the biography second, then use the permanent user number as the durable reference. The permanent key is deliberately isolated in a compact elevated panel and set in data typography so it cannot be mistaken for the Handle.

**Key Characteristics:**

- Teal is a sparse trust and action signal, never ambient decoration.
- Warm light surfaces and cool dark surfaces preserve clear tonal layering in both modes.
- Space Grotesk gives identity headings structure; DM Sans keeps prose approachable.
- Rounded cards, circular avatars, fine rings, and restrained shadows create soft but precise boundaries.
- Cover, avatar, identity copy, permanent key, and public links form a deliberate verification sequence.

## Colors

The palette combines a dependable teal accent with quiet stone-like neutrals and theme-aware semantic surfaces.

### Primary

- **Trust Teal:** Used for primary actions, identity and security icons, focus indicators, active emphasis, and the soft profile-cover fallback.

### Neutral

- **Page Canvas:** The uninterrupted application background behind cards and regions.
- **Credential Card:** The primary reading surface for profiles, settings, and authentication forms.
- **Raised Inset:** A tonal step used for permanent keys, metrics, icon wells, and compact supporting information.
- **Reading Ink:** Default body copy with comfortable contrast.
- **Identity Ink:** Names, headings, and values that require immediate recognition.
- **Supporting Ink:** Handles, labels, hints, timestamps, and secondary explanations.
- **Quiet Boundary:** Fine card rings, dividers, input outlines, and header separation.

### Named Rules

**The Trust Signal Rule.** Teal marks trust, action, focus, or current state; it does not flood large reading surfaces.

**The Semantic Surface Rule.** Use the shared page, card, elevated, text, and border roles so light and dark modes remain equivalent rather than styling either mode independently.

## Typography

**Display Font:** Space Grotesk (with DM Sans and system sans-serif fallbacks)
**Body Font:** DM Sans (with system sans-serif fallback)
**Label/Mono Font:** The platform UI monospace stack for permanent keys and recovery data

**Character:** The pairing is compact and quietly technical. Geometric display headings establish authority while the body face stays neutral and conversational; monospace is reserved for information whose exact characters matter.

### Hierarchy

- **Display** (semibold, 1.5rem; 1.875rem from the small breakpoint, compact line height): Public display names and the most important identity heading.
- **Title** (semibold, 1.125rem): Section headings such as public links.
- **Body** (regular, 1rem, 1.75 line height): Biography and primary explanatory text, held to approximately 68 characters per line.
- **Label** (medium, 0.75rem): Permanent-key labels, hints, metadata, and compact status copy.
- **Data** (semibold, 0.875rem): Permanent user keys and other exact, copy-sensitive values.

### Named Rules

**The Exact Characters Rule.** Use monospace only when a string is a stable identifier, recovery value, or other character-sensitive datum; Handle and prose remain in the body face.

## Layout

Public and account content share a centered, single-column shell. The application frame tops out at 48rem with 1rem side padding; primary profile and settings content narrows to 42rem. The sticky 4rem header maintains orientation while the main region uses generous 2rem to 2.5rem vertical padding.

Spacing follows a compact 0.5rem-based rhythm, with 0.75rem for related controls, 1rem for standard group separation, 1.5rem for card internals, and 2rem between major sections. At the 40rem breakpoint, cover height grows from 9rem to 12rem, the identity row changes from stacked to side-by-side, the permanent-key panel becomes a fixed 13rem sidebar, and public links become a two-column grid. Mobile reading order remains name, Handle, permanent key, biography, then links.

**The Credential Sequence Rule.** In public identity views, preserve the cover-to-avatar anchor and keep the stable key adjacent to—yet visibly separate from—the mutable name and Handle.

## Elevation & Depth

Depth is mostly tonal and structural. Fine one-pixel rings define cards, elevated fills distinguish inset information, and a low ambient shadow is reserved for soft lift such as the avatar and shared cards. Dark mode replaces the light diffuse shadow with a border-like edge or the shared dark card shadow rather than forcing a light-mode glow.

### Shadow Vocabulary

- **Soft Lift** (`0 6px 18px rgb(40 30 60 / 0.06)`): Gentle light-mode lift for shared cards and prominent circular media.
- **Card Edge** (`0 1px 2px rgb(15 23 42 / 0.04)`): Minimal structural separation on Foundation cards.
- **Dark Card Lift** (`inset 0 1px 0 rgb(255 255 255 / 0.03), 0 14px 34px rgb(0 0 0 / 0.18)`): Dark-mode overlay and card depth where a raised surface is required.

### Named Rules

**The Tonal-First Rule.** Establish hierarchy with semantic surface steps and quiet rings before adding shadow.

## Shapes

The form language is soft and functional: standard controls and inset panels use gently curved corners, feature wells use a slightly larger curve, primary cards use a broad 1rem corner, and avatars remain fully circular. Borders are thin and low-contrast. Cover and card edges clip media cleanly; the avatar overlaps the cover inside a background-colored circular buffer so the silhouette remains legible in both modes.

**The One Soft Geometry Rule.** Combine rounded rectangles and circles consistently; avoid sharp containers, ornamental blobs, and mixed corner personalities.

## Components

### Buttons

- **Shape:** Compact rounded controls with medium-weight labels; large actions use 0.5rem by 0.75rem padding.
- **Primary:** Trust Teal fill with inverted text for the principal action in a form or flow.
- **Hover / Focus:** Color changes use the fast shared transition; keyboard focus receives a visible teal outline and offset.
- **Neutral Outline:** Credential links and secondary actions use the card surface, a quiet inset ring, and an elevated-surface hover state.
- **Ghost / Link:** Low-emphasis actions stay transparent until hover and never compete with the principal action.

### Cards / Containers

- **Corner Style:** Broad, soft corners for primary cards; slightly tighter corners for nested panels.
- **Background:** Card surface over the page canvas; elevated surface for insets.
- **Shadow Strategy:** Tonal-first with a fine ring; use Soft Lift only when separation needs reinforcement.
- **Border:** One-pixel semantic boundary or inset ring.
- **Internal Padding:** 1rem on compact screens and 1.5rem at the small breakpoint for shared cards.

### Inputs / Fields

- **Style:** Card-surface field, highlighted text, quiet inset ring, and compact rounded corners.
- **Focus:** The ring shifts to the semantic focus color and a visible outline remains available to keyboard users.
- **Error / Disabled:** Error adopts the semantic error role; disabled fields retain their structure at reduced opacity.

### Navigation

The global header is sticky, 4rem high, lightly translucent, and separated by a quiet bottom border. The brand lockup pairs a teal-tinted rounded icon well with a semibold display label; utilities remain compact on the right. Back-to-top and menu controls use the same neutral surface, rounded geometry, and visible focus treatment.

### Public Identity Credential

The profile card begins with a full-width cover, then a circular avatar overlapping its lower edge. Display name and Handle form the mutable identity block; the permanent user key occupies its own raised sidebar with a link icon and monospace value. Biography follows in a readable measure. Public links appear only when supplied and use full-width neutral outline actions with platform and external-link icons. A small shield footer identifies Account as the source without implying verification or endorsement.

## Do's and Don'ts

### Do:

- **Do** preserve semantic color roles and test every surface in light and dark modes.
- **Do** keep display name and Handle visually grouped while isolating the permanent user key in data typography.
- **Do** retain the cover-over-avatar composition for identity headers and keep biography lines near the established readable measure.
- **Do** provide visible keyboard focus and a logical mobile reading order for every interactive identity element.
- **Do** render only user-supplied public fields and links.

### Don't:

- **Don't** turn a public profile into a social analytics dashboard or invent counts, badges, or endorsement signals.
- **Don't** present Handle and permanent user key with equal styling or imply that both are immutable.
- **Don't** expose email, role, internal UUID, account state, or security information on public surfaces.
- **Don't** use teal as a large decorative wash when a neutral semantic surface communicates the hierarchy.
- **Don't** introduce sharp cards, heavy shadows, or a second icon family into the Account visual world.
