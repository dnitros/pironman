# Pironman

CLI and daemon for the Pironman 5 case (base edition) on a Raspberry Pi 5. It controls the RGB strip, the OLED display and the case fan, and handles the power button.

## Requirements

- Raspberry Pi 5 with a systemd-based OS.
- SPI and I2C enabled: `sudo raspi-config` → Interface Options, or `dtparam=spi=on` and `dtparam=i2c_arm=on` in `/boot/firmware/config.txt`.
- Go 1.27.1 or later to build.

## Install

On the Pi:

```sh
make install   # build, copy to /usr/local/bin, install, enable and start the service
```

`daemon install` adds you to the `pironman` group, which can use the control socket without `sudo`. Log out and back in for it to take effect. If it can't add you, it prints the `usermod` command to run.

## Update

The service already points at `/usr/local/bin/pironman`, so an update only replaces the binary and restarts the service:

On the Pi:

```sh
make update   # rebuild, replace the binary, restart the service, print the version
```

## Usage

| Command | Does |
|---|---|
| `pironman status` | Show RGB, OLED, case fan and PWM fan state |
| `pironman doctor` | Check the daemon, service, socket and config |
| `pironman version` | Show the build's git SHA |
| `pironman rgb on\|off` | Turn the RGB strip on or off |
| `pironman rgb color <#hex>` | Set a solid color |
| `pironman rgb brightness <0-100>` | Set brightness |
| `pironman rgb style <name> [--speed 0-100]` | `solid`, `breathing`, `flow`, `flow_reverse`, `rainbow`, `rainbow_reverse`, `hue_cycle`, `flow_fade`, `flow_fade_reverse` |
| `pironman oled on\|off` | Wake the display on the `mix` page, or blank it |
| `pironman oled page <name\|next\|prev>` | Show a page from `page_order` |
| `pironman oled image [--interval s] [--invert] <path>...` | Show `.png`, `.jpg` or 128x64 `.pbm` images on the `image` page |
| `pironman oled rotation <0\|180>` | Flip the display |
| `pironman oled sleep-timeout <seconds>` | Blank after this long idle, 0–3600 (`0` never blanks) |
| `pironman fan mode <name>` | `always_on`, `performance`, `cool`, `balanced`, `quiet` |
| `sudo pironman daemon <install\|uninstall\|start\|stop\|enable\|disable>` | Manage the systemd service |

Power button: click shows the next OLED page, double-click shows the previous one. Hold it to show a shutdown prompt, then release to shut the Pi down.

Stopping the daemon turns the RGB strip and case fan off. They return to their saved state when it starts again.

## Configuration

`/etc/pironman/config.yaml`, or the path in `PIRONMAN_CONFIG_PATH`. CLI commands update it. The defaults, used when the file doesn't exist:

```yaml
rgb:
  enabled: true
  color: "#00ff00"
  brightness: 80
  style: solid
  speed: 50
oled:
  enabled: true
  sleep_timeout_seconds: 10
  scroll_interval_seconds: 3
  page_order: [mix, performance, ips, disk]   # add "image" to use oled image
  image_interval_seconds: 5
  rotation: 0
fan:
  case_fan_state: balanced
```

Images from `oled image` are stored in `images/` next to the config file. Changes to `page_order` take effect when the daemon restarts.

## Development

```sh
make build
make test
make vet
make         # list all targets
```

The daemon needs the real hardware, so the hardware-facing packages are tested against fakes. To try CLI commands without root, point them at another socket:

```sh
PIRONMAN_SOCKET_PATH=/tmp/pironman.sock ./pironman doctor
```
