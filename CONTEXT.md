# Pironman

A CLI + daemon tool controlling a Pironman 5 case (base edition) on a Raspberry Pi 5.

## Language

**OLED page**:
One of the fixed full-screen contents the OLED display can show: `mix`, `performance`, `ips`, `disk`.
_Avoid_: screen, view

**Page advance**:
Switching the OLED from one page to another. Triggered only by a power-button press (short press = next, double press = previous) — never by a timer.
_Avoid_: auto-cycle, page rotation

**Content scroll**:
Cycling automatically, on a fixed timer, through multiple values displayed *within* a single page (e.g. several IP addresses on the `ips` page, or several disks on the `disk` page). Distinct from page advance: a page's content scrolls on its own while the page itself only changes on a button press.
_Avoid_: auto-cycle, page rotation

**Press event**:
One of four classified power-button interactions, each mapped to a fixed action: `click` (OLED wake/page-advance-next), `double-click` (page-advance-previous), `long-press` (show shutdown-confirmation screen), `long-press-released` (shutdown).
_Avoid_: button press, tap

**PWM fan**:
The Raspberry Pi 5's own official active-cooler fan, governed by the Pi's own kernel/firmware thermal management. This tool only ever reads its state and speed via sysfs — it never writes to it.
_Avoid_: cooling fan, the fan

**Case fan**:
The Pironman case's own fan, wired to a GPIO relay. This tool actively controls it (on/off in v1, temperature-gated modes in v2) — the Pi's own thermal governor has no say over it.
_Avoid_: GPIO fan, the fan
