# Research: OLED icon/font pixel-parity options (PER-20)

Scope: whether/how to layer bitmap icons and a custom font on top of the
text-only OLED baseline shipped in PER-19 (`basicfont`, `golang.org/x/image`,
zero assets). That baseline is not revisited here — this is pure research for
a possible follow-up.

## 1. How `sunfounder/pm_auto` bundles and draws its icons and font

Repo: [github.com/sunfounder/pm_auto](https://github.com/sunfounder/pm_auto), branch `v2`.

- **Rendering path**: confirmed Pillow (`from PIL import Image, ImageDraw, ImageFont`).
  The OLED wrapper is `pm_auto/libs/ssd1306.py`, a `SSD1306` class wrapping an
  Adafruit-derived `SSD1306_128_64` I2C driver (`pm_auto/libs/ssd1306.py`,
  header credits "Copyright (c) 2014 Adafruit Industries, Author: Tony DiCola",
  MIT-licensed driver code embedded in the same file). It keeps a Pillow
  `Image.new('1', (128, 64))` canvas plus an `ImageDraw.Draw` handle, and
  `display()` converts that 1-bit image to the SSD1306 page/column byte buffer
  and writes it over I2C.
- **Icon drawing** (`SSD1306.draw_icon`, in `pm_auto/libs/ssd1306.py`): opens
  the PNG with Pillow, converts to RGBA, optionally rescales, composites the
  alpha channel onto a black background, then converts to 1-bit either via
  Floyd–Steinberg dithering (`Image.Dither.FLOYDSTEINBERG`, the default,
  `dither=True`) or a hard threshold (`dither=False, threshold=N`, used for
  the "DISCONNECTED" error icon and network icons in `mix.py` with per-icon
  threshold values 50/80/85). The result is pasted onto the canvas at
  arbitrary (x, y).
- **Text drawing** (`SSD1306.draw_text`): `ImageFont.truetype(font_path, size)`
  loaded per call (no caching), then `ImageDraw.text(...)`. Size is passed as
  a font point size per call site (e.g. `mix.py` uses 14pt for the big
  numbers, 10pt for labels), i.e. one TTF is rasterized at multiple sizes at
  runtime, not pre-rendered bitmaps per size.
- **Icon assets**: `pm_auto/assets/icons/*.png`. Fetched and inspected two
  representative files directly:
  - `icon_cpu_24.png`: 24×24, 8-bit RGBA, non-interlaced (230 bytes on disk).
  - `icon_network_20.png`: 20×20, 8-bit RGBA, non-interlaced (512 bytes on
    disk).
  Other icons in the directory follow the same `icon_<name>_<size>.png`
  convention at 16/20/24/40/48px, all PNG/RGBA (list: battery, charge, cpu,
  error, fan, hard_drive, lan, network, plug, raid, ram, sd_card,
  temperature, usb_disk, wifi, raspberry, sunfounder — 18 icon files total).
  These are true grayscale-with-alpha bitmaps (anti-aliased edges), not
  1-bit source art — the 1-bit conversion happens at draw time as described
  above.
- **Font assets**: `pm_auto/assets/fonts/` contains three TTFs:
  `UbuntuSans-Regular.ttf` (414,232 bytes — the one used by the OLED pages
  and the default in `SSD1306.init()`), `UbuntuMono-Regular.ttf` (189,892
  bytes), `Minecraftia-Regular.ttf` (70,324 bytes, a pixel-art font, unused by
  the OLED code paths inspected).
- **Font license**: the repo itself (`pm_auto/LICENSE`) is **GPLv2**, which
  is a separate, larger consideration than just the font — anything copied
  or closely derived from this repo's Python source is GPLv2-encumbered,
  independent of the font question. For the font specifically:
  `UbuntuSans-Regular.ttf`'s embedded copyright string (extracted via
  `strings` on the binary) references `copyright.c2sc`, consistent with it
  being one of Canonical's fonts under the **Ubuntu Font Licence (UFL) 1.0**
  — [full text](https://assets.ubuntu.com/v1/81e5605d-ubuntu-font-licence-1.0.txt).
  Key clauses:
  - Permits exactly this use: *"The fonts, including any derivative works,
    can be bundled, embedded, and redistributed provided the terms of this
    licence are met."*
  - Requires the font itself to stay under UFL 1.0 even when embedded:
    *"The Font Software, modified or unmodified, in part or in whole, must be
    distributed entirely under this licence, and must not be distributed
    under any other licence."* This does **not** require the surrounding
    program (the Go daemon) to be GPL/UFL-licensed — the UFL, like the SIL
    OFL it's modeled on, is designed to let the font travel inside
    differently-licensed software, only the font asset itself must carry UFL
    terms (e.g. a `LICENSE-UbuntuSans.txt` alongside the embedded `.ttf`).
  - No restriction on commercial use or embedding in a compiled/closed
    binary.
  - **Verdict**: embedding `UbuntuSans-Regular.ttf` (or downloading it fresh
    from Canonical/Google Fonts rather than copying pm_auto's copy) is
    permitted for this project, provided the UFL 1.0 text is shipped
    alongside it. The bigger blocker for reuse is not the font — it's that
    pm_auto's *code* (icon compositing logic, page layouts, driver) is GPLv2,
    so any Go port should treat pm_auto as a read-only reference for pixel
    dimensions/behavior, not as source to copy from, to avoid GPL
    contamination of this project's license.

## 2. Does `l-you/pironman5-go` implement OLED icons?

Repo: [github.com/l-you/pironman5-go](https://github.com/l-you/pironman5-go),
branch `main`. It has OLED code (`internal/oled/oled.go`,
`internal/hardware/ssd1306.go`) and **went text-only, no icons, no custom
font** — the same direction as this project's PER-19 baseline.

Evidence, `internal/oled/oled.go`:
- Imports `golang.org/x/image/font`, `golang.org/x/image/font/basicfont`,
  `golang.org/x/image/math/fixed` — identical building blocks to this
  project's chosen approach.
- `drawText()` (bottom of the file) draws with a hardcoded
  `basicfont.Face7x13` face via `font.Drawer`, no size variation, no TTF.
- Bars/rects/a heart icon are all drawn with raw pixel loops
  (`drawRect`/`fillRect`/`renderHeart`) against an `image.Gray` buffer — e.g.
  `renderHeart` is a hand-authored 9×8 bitmap encoded as a `[]string` of
  `"0"`/`"1"` rows, scaled ×5 and blitted with `fillRect`. This is the
  "hand-drawn bitmap, no font rendering" pattern referenced in question 3
  below, applied to exactly one icon (a heartbeat/alive indicator), not to
  the stat icons (network/CPU/temp/RAM/fan/disk) that pm_auto has bitmaps
  for.
- It does support one image-like feature beyond plain text: a user-supplied
  "image" OLED page (`config.OLEDPageImage`) that loads an arbitrary
  pre-converted 128×64 1-bit `.pbm` file from disk (`internal/pbm`,
  `loadImage`/`pbm.DecodeFile` in `oled.go`) and blits it directly
  (`drawImage`/`draw.Draw`). That's a user-content slideshow feature, not
  icon/font pixel parity with pm_auto's stat pages.

So the finding is unambiguous: this other Go port did not attempt icon or
custom-font parity with pm_auto; it independently converged on the same
`basicfont`-only approach this project already chose for PER-19, plus a
single hand-bitmapped decorative icon and a raw-pixel user-image slideshow
feature.

## 3. Go-side options for embedding and rendering the assets

All three options can coexist with the PER-19 text baseline (add icons next
to `basicfont` labels) rather than requiring a rewrite.

**`go:embed` for icon bitmaps**: uncontroversial, stdlib since Go 1.16 —
embeds a `[]byte` or `embed.FS` at compile time, no runtime file I/O, no
extra dependency. This is the mechanism for whichever bitmap-asset option
below is chosen; it does not itself decide font-vs-sprite.

**Option A — embedded TTF + `x/image/font/opentype`/`sfnt` rasterization**
(closest to what pm_auto/Pillow does):
- Binary size: pm_auto's own `UbuntuSans-Regular.ttf` is 414 KB; a subsetted
  version (Latin + digits + a handful of symbols only, via a subsetting tool)
  would likely land in the tens-of-KB range, but no measured subset size was
  produced in this research — stated as an estimate, not a benchmark.
- CPU cost: rasterizes (antialiases) glyph outlines to a bitmap on every
  distinct (glyph, size) pair requested. `sfnt`/`opentype` do not appear to
  cache rasterized glyphs across calls by default (per the package docs
  surfaced in this research — see
  [pkg.go.dev/golang.org/x/image/font/opentype](https://pkg.go.dev/golang.org/x/image/font/opentype)
  and [pkg.go.dev/golang.org/x/image/font/sfnt](https://pkg.go.dev/golang.org/x/image/font/sfnt)),
  so a naive re-render on every OLED refresh re-rasterizes every glyph. This
  is solvable with an app-level glyph cache, but that's exactly the kind of
  added complexity this option carries.
  code complexity is the highest of the three: font loading/parsing error
  handling, an `opentype.NewFace` per size actually needed, and (if adopted)
  a glyph-bitmap cache to avoid re-rasterizing every frame.

**Option B — pre-rendered/pre-rasterized custom bitmap font** (a hand-built
"basicfont-shaped" replacement matching UbuntuSans's letterforms at the
specific sizes PER-19 needs, generated once offline):
- Binary size: much smaller than an embedded TTF — a fixed per-glyph bitmap
  table at one or two sizes (comparable in spirit to `basicfont.Face7x13`,
  which is itself only a few KB of Go source) costs roughly (glyph count) ×
  (bitmap bytes per glyph), typically low single-digit KB per size.
- CPU cost: lowest — blitting precomputed 1-bit glyph bitmaps is a memcpy-
  style loop, same cost model as `basicfont` already in use.
- Complexity: moderate — no runtime TTF parsing, but requires an offline
  generation step (rasterize the reference font once, hand-tune kerning/
  spacing, commit the generated Go table) and ongoing maintenance if new
  glyphs/sizes are needed later.

**Option C — hand-drawn bitmap icons at the original's exact pixel
dimensions, no font rendering at all** (icons only, blitted sprites; text
stays on `basicfont` as PER-19 already does):
- Binary size: smallest of the three — a handful of 16–48px monochrome
  sprites (matching pm_auto's `icon_*_NN.png` dimensions: 16/20/24/40/48px)
  as `go:embed`-ed 1-bit bitmaps or Go byte-array literals, each on the order
  of tens to a few hundred bytes.
- CPU cost: lowest — direct blit, no rasterization, no font handling at all.
- Complexity: lowest — no font library dependency, no glyph cache, just a
  fixed lookup table of icon name → bitmap, and a blit function (which
  `l-you/pironman5-go`'s `fillRect`/fixed-bitmap `renderHeart` pattern already
  demonstrates as viable in this exact codebase family).

## 4. RP1/Pi 5 I2C latency concern

Research found **I2C-specific evidence of degraded reliability/throughput on
the RP1 southbridge**, but **no specific latency-in-microseconds benchmark**
for a redraw workload (icon blits + antialiased text) at 100kHz/400kHz I2C.
Stating both plainly rather than inventing a number:

- Confirmed I2C-specific issues on Pi 5/RP1 in the upstream kernel tracker:
  - [`raspberrypi/linux#5792`](https://github.com/raspberrypi/linux/issues/5792):
    i2c1 on Pi 5 caps out around 650kHz even when the device-tree overlay
    requests 1MHz, whereas the same overlay produces a true 1MHz clock on a
    CM4 (non-RP1 SoC-direct I2C). This indicates the RP1-mediated I2C
    controller does not reach the same clock rates as SoC-direct I2C on
    earlier Pi models.
  - [`raspberrypi/linux#5784`](https://github.com/raspberrypi/linux/issues/5784):
    reports of "controller timed out" / "i2c_dw_handle_tx_abort: lost
    arbitration" errors on Pi 5 I2C requiring a power cycle to recover —
    a reliability concern, not purely a latency number, but relevant to any
    design that increases per-frame I2C write volume (larger, more frequent
    icon+font redraws vs. today's plain text).
  - General RP1 architecture: RP1 is reached over a PCIe Gen 2 x1 link from
    the BCM2712 SoC rather than being on-die, per
    [PiCockpit's RP1 documentation summary](https://picockpit.com/raspberry-pi/i-read-the-rp1-documentation-so-you-dont-have-to/)
    and discussion in
    [`geerlingguy/sbc-reviews#21`](https://github.com/geerlingguy/sbc-reviews/issues/21),
    which puts a structural latency floor (PCIe round-trip plus mailbox/
    interrupt handling) under every RP1-mediated peripheral access, GPIO
    included, that direct-attached peripherals on earlier Pis didn't have.
    That issue's figures (10–50µs GPIO toggle variance, ~1µs typical PCIe
    hop) are about GPIO, not I2C directly, and are cited here only as
    evidence of the general architecture, not as an I2C number.
- **No benchmark was found** measuring end-to-end SSD1306 frame-write time on
  Pi 5/RP1 specifically, nor any comparison of "plain text redraw" vs.
  "icon+antialiased-text redraw" I2C transaction time. Given the SSD1306
  driver writes the frame buffer in 16-byte I2C block writes regardless of
  what was drawn into the buffer (confirmed in both
  `pm_auto/libs/ssd1306.py`'s `display()` and `l-you/pironman5-go`'s
  `SSD1306Display.Display` in `internal/hardware/ssd1306.go`), the **I2C
  transfer cost for a full 128×64 frame is fixed at 1024 bytes regardless of
  whether the pixels were produced by icons+TTF or plain text** — pixel
  content doesn't change the wire cost, only the CPU-side render cost does.
  So the RP1 I2C throughput/reliability concerns above bound on *how often*
  and *how reliably* a fixed-size frame can be pushed, not on whether icons
  make each push larger.

## Recommendation

**Option C — hand-drawn bitmap icons at fixed pixel dimensions, blitted with
no font-rendering library, layered on top of the existing `basicfont` text.**

This repo's engineering culture (CLAUDE.md, ponytail guidance) favors the
simplest approach that works and treats new dependencies as unjustified
unless proven necessary. Weighing the three options:

- Option A (embedded TTF + `opentype`/`sfnt`) buys the most visual fidelity
  (arbitrary point sizes, matches pm_auto exactly) but at the highest cost:
  a ~400KB-class font asset (or an unverified subsetting effort to shrink
  it), a new rendering dependency surface (`x/image/font/opentype` or
  `sfnt`, not currently used by this project beyond the already-adopted
  `basicfont`), and a real CPU cost per frame unless a glyph cache is also
  built — that's two pieces of new complexity (font parsing + caching) to
  solve a problem (readable stat labels) PER-19's plain-text approach
  already solves. `l-you/pironman5-go` independently reached the same
  conclusion and skipped it entirely.
- Option B (custom pre-rendered bitmap font) sits in between: smaller and
  faster than A, but it duplicates what `basicfont` already provides (a
  fixed-size bitmap font) for the sole benefit of matching UbuntuSans's
  letterforms — a purely cosmetic gain that doesn't change what information
  is on screen, and needs an offline generation/maintenance step for no
  functional payoff.
  Skipped: replacing `basicfont` with a custom bitmap font. Add if a future
  ticket explicitly asks for typographic parity with the Python original;
  today nothing requests it.
- Option C is the only one that adds something `basicfont`-only text can't
  do at all — glanceable icons (network type, disk type, error state) — for
  the lowest cost: no new dependency, no font-rendering code path, sizes
  matching pm_auto's own 16–48px references, and the exact blit pattern
  (`fillRect`-style pixel loop) `l-you/pironman5-go` already validated in
  this same codebase family for its heart icon. It composes cleanly with the
  PER-19 baseline (icons sit beside existing text labels, nothing about the
  current renderer needs to change).

Recommended follow-up scope, if this ticket becomes real work later: pick
the 3–5 icons that carry information `basicfont` text can't (e.g. wifi vs.
ethernet, error/disconnected state) rather than porting all 18 of pm_auto's
icons, hand-draw each at pm_auto's reference pixel size as a 1-bit
`go:embed`-ed bitmap, and skip the font question entirely — `basicfont`
stays.
