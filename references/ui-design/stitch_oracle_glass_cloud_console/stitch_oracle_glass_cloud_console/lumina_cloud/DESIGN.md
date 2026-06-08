---
name: Lumina Cloud
colors:
  surface: '#faf8fe'
  surface-dim: '#dad9df'
  surface-bright: '#faf8fe'
  surface-container-lowest: '#ffffff'
  surface-container-low: '#f4f3f8'
  surface-container: '#eeedf3'
  surface-container-high: '#e9e7ed'
  surface-container-highest: '#e3e2e7'
  on-surface: '#1a1b1f'
  on-surface-variant: '#5c403c'
  inverse-surface: '#2f3034'
  inverse-on-surface: '#f1f0f5'
  outline: '#916f6a'
  outline-variant: '#e6bdb7'
  surface-tint: '#be0f0e'
  primary: '#bb0c0d'
  on-primary: '#ffffff'
  primary-container: '#e02e24'
  on-primary-container: '#fffeff'
  inverse-primary: '#ffb4a9'
  secondary: '#0058bc'
  on-secondary: '#ffffff'
  secondary-container: '#0070eb'
  on-secondary-container: '#fefcff'
  tertiary: '#006295'
  on-tertiary: '#ffffff'
  tertiary-container: '#007cbb'
  on-tertiary-container: '#fffeff'
  error: '#ba1a1a'
  on-error: '#ffffff'
  error-container: '#ffdad6'
  on-error-container: '#93000a'
  primary-fixed: '#ffdad5'
  primary-fixed-dim: '#ffb4a9'
  on-primary-fixed: '#410001'
  on-primary-fixed-variant: '#930004'
  secondary-fixed: '#d8e2ff'
  secondary-fixed-dim: '#adc6ff'
  on-secondary-fixed: '#001a41'
  on-secondary-fixed-variant: '#004493'
  tertiary-fixed: '#cce5ff'
  tertiary-fixed-dim: '#92ccff'
  on-tertiary-fixed: '#001d31'
  on-tertiary-fixed-variant: '#004b73'
  background: '#faf8fe'
  on-background: '#1a1b1f'
  surface-variant: '#e3e2e7'
typography:
  display-lg:
    fontFamily: Inter
    fontSize: 48px
    fontWeight: '700'
    lineHeight: 56px
    letterSpacing: -0.02em
  headline-lg:
    fontFamily: Inter
    fontSize: 32px
    fontWeight: '600'
    lineHeight: 40px
    letterSpacing: -0.01em
  headline-md:
    fontFamily: Inter
    fontSize: 24px
    fontWeight: '600'
    lineHeight: 32px
    letterSpacing: -0.01em
  headline-sm:
    fontFamily: Inter
    fontSize: 20px
    fontWeight: '600'
    lineHeight: 28px
    letterSpacing: '0'
  body-lg:
    fontFamily: Inter
    fontSize: 17px
    fontWeight: '400'
    lineHeight: 24px
    letterSpacing: -0.01em
  body-md:
    fontFamily: Inter
    fontSize: 15px
    fontWeight: '400'
    lineHeight: 20px
    letterSpacing: '0'
  label-md:
    fontFamily: Inter
    fontSize: 13px
    fontWeight: '500'
    lineHeight: 16px
    letterSpacing: 0.01em
  label-sm:
    fontFamily: Inter
    fontSize: 11px
    fontWeight: '600'
    lineHeight: 14px
    letterSpacing: 0.03em
rounded:
  sm: 0.25rem
  DEFAULT: 0.5rem
  md: 0.75rem
  lg: 1rem
  xl: 1.5rem
  full: 9999px
spacing:
  unit: 4px
  container-padding-desktop: 40px
  container-padding-mobile: 20px
  gutter: 24px
  stack-sm: 8px
  stack-md: 16px
  stack-lg: 32px
---

## Brand & Style
The design system is engineered for the next generation of cloud infrastructure management, blending the rigorous utility of enterprise software with the refined aesthetics of high-end consumer operating systems. The brand personality is **sophisticated, intentional, and high-fidelity**, prioritizing clarity and cognitive ease.

The style is rooted in **Modern Minimalism with Glassmorphic layering**. It leverages depth through translucency rather than heavy shadows, creating a sense of "lightness" despite the complexity of cloud data. The emotional response should be one of total control and premium reliability, echoing the tactile yet digital feel of modern desktop interfaces.

## Colors
The palette is dominated by **Pure Whites** and **Subtle Greys** to provide a clean canvas for complex data visualization. 
- **Primary:** Oracle Red is used exclusively for critical status indicators, primary actions, and brand identification.
- **Secondary:** An iOS-inspired Blue is utilized for interactive elements like links and multi-select states to differentiate from destructive actions.
- **Surface Strategy:** Backgrounds utilize a subtle off-white (`#F5F5F7`) to reduce eye strain, while active surfaces use glassmorphism with a `20px` backdrop blur to maintain spatial awareness.

## Typography
The typography uses **Inter** as a proxy for SF Pro, emphasizing a systematic and neutral tone.
- **Headlines:** Use tighter tracking and semi-bold weights to mimic the "Display" variants of the Apple type system.
- **Body Text:** Set at 15px/17px for optimal readability in data-dense cloud environments.
- **Labels:** Utilize slightly increased letter spacing for small-cap or uppercase utility text to ensure legibility in technical metadata.

## Layout & Spacing
This design system employs a **Fixed-Fluid Hybrid** layout. Sidebars and navigation panels are fixed-width with frosted-glass backgrounds, while the main content area utilizes a fluid 12-column grid.

- **Margins:** A generous 40px margin on desktop ensures the UI feels premium and uncrowded.
- **Rhythm:** An 8pt spacing system is the baseline, but "breathable" sections (like dashboard overviews) should use 32px or 48px increments to maintain the minimalist aesthetic.
- **Breakpoints:** 
  - Mobile: 0-767px (1-column stack)
  - Tablet: 768-1199px (Compressed sidebar)
  - Desktop: 1200px+ (Full expanded glass navigation)

## Elevation & Depth
Depth is expressed through **translucency and layering** rather than traditional dropshadows.
- **Level 1 (Base):** The main background.
- **Level 2 (Panels):** Semi-transparent glass (`backdrop-filter: blur(20px)`) with a 1px inner white border (at 10% opacity) to catch light.
- **Level 3 (Modals/Popovers):** Higher saturation of the glass effect with a soft, 64px spread ambient shadow (`rgba(0,0,0,0.08)`) to lift it off the surface.
- **Interaction:** Hovering over elements should produce a subtle "lift" effect using a slightly brighter background color rather than a shadow increase.

## Shapes
The shape language is defined by **large, friendly radii** that soften the technical nature of cloud computing. 
- **Standard Cards:** 16px to 24px corner radius.
- **Buttons:** Fully rounded (pill-shaped) for secondary actions, or 12px for primary actions to maintain a professional weight.
- **Inputs:** 8px to 10px radius to provide a slight contrast to the more rounded containers.

## Components
- **Buttons:** Primary buttons use the Oracle Red gradient. Secondary buttons use a "ghost" style with a thin border or a light grey frosted-glass background.
- **Inputs:** Fields are defined by a subtle 1px border and a light grey fill. On focus, the border glows with a soft blue ring.
- **Cards:** Use 24px padding and a 20px border radius. Backgrounds are either pure white (light mode) or translucent black (dark mode).
- **Chips/Status:** Small, pill-shaped indicators with low-opacity background tints (e.g., green tint for "Running") and high-contrast text.
- **Sidebar:** A persistent glassmorphic panel on the left with SF-style iconography. Icons should be `20px` in size with a `Medium` weight.
- **Data Tables:** High-density text but with generous row heights (48px+) and no vertical borders to maintain the "clean" aesthetic.