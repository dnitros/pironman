# Pironman

CLI and daemon controlling a Pironman 5 case (base edition) on a Raspberry Pi 5.

## Language

**OLED page**:
A full-screen content the OLED shows: `mix`, `performance`, `ips`, `disk`, `image`.
_Avoid_: screen, view

**Page advance**:
Switching from one OLED page to another, only by power-button press (click = next, double-click = previous). Never by a timer.
_Avoid_: auto-cycle, page rotation

**Content scroll**:
Cycling on a timer through several values within one page: IPs on `ips`, LAN and Tailscale addresses on `mix`, disks on `disk`, images on `image`. The page itself doesn't change.
_Avoid_: auto-cycle, page rotation

**OLED rotation**:
The display's orientation, `0` or `180`, set by `pironman oled rotation` and stored as `config.OLED.Rotation`. It flips every page's pixels and has nothing to do with which page or content is shown.
_Avoid_: page rotation

**Press event**:
A classified power-button interaction: `click` (wake, next page), `double-click` (previous page), `long-press` (shutdown prompt), `long-press-released` (powering-off screen, then shutdown).
_Avoid_: button press, tap

**PWM fan**:
The Pi 5's own active cooler, controlled by the kernel. This tool only reads its state and speed from sysfs.
_Avoid_: cooling fan, the fan

**Case fan**:
The case's two fans on one GPIO relay, switched together by one of five curves: `always_on`, `performance`, `cool`, `balanced`, `quiet`. The fans' built-in RGB shares their power, so it turns on and off with them.
_Avoid_: GPIO fan, the fan

**RGB strip**:
The 4 WS2812 LEDs on the main board (SPI0/GPIO10), driven by `pironman rgb`. Separate from the case fans' built-in RGB.
_Avoid_: fan lights, RGB fan
