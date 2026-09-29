# Research: disk-partition dedup by physical device (PER-28)

Scope: PER-25 shipped a disk page (`internal/sysstats.ProcSource.disksFromMounts`
and `classifyDiskType`, plus `internal/oled`'s `diskLines`, on the
`tool/per-19-phase-2-oled-base-page-state-machine-system-stats` branch — not
yet merged to `main` at the time of this research) that turns every
`/dev/`-backed line of `/proc/mounts` into its own disk row, with no logic
grouping partitions of the same physical device. On a stock Raspberry Pi OS
install the SD card alone produces two such lines (`/` on `/dev/mmcblk0p2`,
`/boot/firmware` on `/dev/mmcblk0p1`), so a single-SD-card Pironman 5 shows 2
rows on a page that only scrolls 3 at a time (`diskLines`'s `groupSize = 3`)
for what is physically one disk. This is pure research — no code changes.

## 1. Linux device naming: partition → parent device (kernel source, authoritative)

Fetched `torvalds/linux` at commit
[`6f8319e`](https://github.com/torvalds/linux/commit/6f8319e3e9a44dd537d17f41565a8453c560a581)
(current `master` at research time).

**The naming rule is generated, not guessed — it's right in the kernel's
partition-creation code.** `block/partitions/core.c`, `add_partition()`
([source](https://github.com/torvalds/linux/blob/6f8319e3e9a44dd537d17f41565a8453c560a581/block/partitions/core.c#L330-L338)):

```c
pdev = &bdev->bd_device;
dname = dev_name(ddev);
if (isdigit(dname[strlen(dname) - 1]))
	dev_set_name(pdev, "%sp%d", dname, partno);
else
	dev_set_name(pdev, "%s%d", dname, partno);
```

`ddev` is the whole-disk device, `dname` its name. The rule: **if the parent
disk's name ends in a digit, the partition name is `<parent>p<N>`; otherwise
it's `<parent><N>`.** This is exactly why:

- `mmcblk0` (ends in `0`, a digit) → `mmcblk0p1`, `mmcblk0p2`
- `nvme0n1` (ends in `1`, a digit) → `nvme0n1p1`
- `sda` (ends in `a`, not a digit) → `sda1`, `sda2`

The same function also confirms **parent linkage is a first-class kernel
concept, not something userspace has to infer**: `pdev->parent = ddev;`
([same block](https://github.com/torvalds/linux/blob/6f8319e3e9a44dd537d17f41565a8453c560a581/block/partitions/core.c#L332)) —
the partition's `struct device` is a child of the whole disk's `struct
device` in the kernel's device model, which is what makes the partition's
sysfs entry live *underneath* the disk's sysfs directory (confirmed by
[sysfs-rules.rst](https://www.kernel.org/doc/html/latest/admin-guide/sysfs-rules.html):
sysfs mirrors the kernel's internal device-model hierarchy).

Cross-checked against the legacy major/minor table,
[`Documentation/admin-guide/devices.txt`](https://github.com/torvalds/linux/blob/6f8319e3e9a44dd537d17f41565a8453c560a581/Documentation/admin-guide/devices.txt):
- Major 179 (MMC block devices): `0 = /dev/mmcblk0`, `1 = /dev/mmcblk0p1`
  ("First partition on first MMC card"), `8 = /dev/mmcblk1` for the next
  card — consistent with the `p<N>` rule above.
- Major 8 (SCSI disk devices): `0 = /dev/sda` ("First SCSI disk whole
  disk"), with partitions "handled in the same way as for IDE disks...
  except that the limit on partitions is 15" — consistent with the bare
  `<N>` rule (`sda1`, not `sda-p1`).
- NVMe has no entry in `devices.txt` at all — it doesn't use the legacy
  static major/minor scheme the file documents, it allocates dynamically.
  That's not a gap in the naming rule, though: `add_partition()`'s
  digit-suffix check is scheme-agnostic and applies to NVMe the same as
  everything else, which is exactly why `nvme0n1p1` follows the `p<N>` form.

**The sysfs `partition` attribute exists and is naming-scheme-agnostic**,
confirmed directly in `block/partitions/core.c`:

```c
static ssize_t part_partition_show(struct device *dev,
				   struct device_attribute *attr, char *buf)
{
	return sysfs_emit(buf, "%d\n", bdev_partno(dev_to_bdev(dev)));
}
...
static DEVICE_ATTR(partition, 0444, part_partition_show, NULL);
...
const struct device_type part_type = {
	.name		= "partition",
	...
};
```

This `DEVICE_ATTR(partition, ...)` is only ever attached to devices of
`part_type` (i.e. partitions, never whole disks), so **the presence of
`/sys/class/block/<dev>/partition` is itself the naming-agnostic signal that
`<dev>` is a partition, not a disk.** The history matches: the original 2008
LKML patch introducing it
([lkml.iu.edu/.../1974.html](https://lkml.iu.edu/hypermail/linux/kernel/0810.1/1974.html))
was written specifically because, with extended device numbers, "subtracting
the minor number from that of the parent device doesn't work anymore" and
the alternative — "parsing the partition name, which is brittle and not
exactly universal" — was explicitly rejected in favor of a real attribute.

Because `pdev->parent = ddev` places every partition's sysfs entry inside its
disk's sysfs directory, resolving `/sys/class/block/<partition>` (a symlink)
and taking the parent directory's basename recovers the physical disk name
without parsing the device name at all — e.g. `readlink -f
/sys/class/block/mmcblk0p1` resolves under `.../block/mmcblk0/mmcblk0p1`.
This is the more robust, scheme-agnostic alternative to string parsing; see
the Recommendation for why it wasn't chosen for this ticket.

## 2. `sunfounder/pm_auto` / `sunfounder/pironman5` — the reference implementation already solves this

**Caveat, restated from PER-20's research
(`docs/research/oled-icon-font-pixel-parity.md`)**: `pm_auto` and
`pironman5` are **GPLv2**. Everything below is read for reference —
understanding what pixel layout/behavior to independently reimplement —
never to be copied into this repo.

Fetched `sunfounder/pironman5` at
[`877f57f`](https://github.com/sunfounder/pironman5/commit/877f57f7b0e663419eec47a6b73d102860d939e6)
(branch `v1`) and `sunfounder/pm_auto` at
[`117f665`](https://github.com/sunfounder/pm_auto/commit/117f66567c6b5b74e588a02fa5c192fc69624b1f)
(branch `v2`, same commit PER-20 researched).

**`pironman5` itself does not implement the disk page.**
`pironman5/pironman5/variants/modules/oled.py` only registers the `disk` OLED
page name and its default config (`oled_page_disk` peripheral, `'disk'` in
`oled_pages`) — no enumeration or rendering logic. The actual implementation
lives in the separately-installed `pm_auto` service (consistent with PER-20's
framing of `pm_auto/libs/ssd1306.py` as the real OLED driver this whole
product uses).

**`pm_auto`'s disk page** (`pm_auto/pm_auto/addons/oled/pages/disks.py`)
iterates `data['disks']`, a dict keyed by physical disk name, drawing up to 3
rows per screen (`islice(disks_info.items(), self.disk_index, self.disk_index
+ 3)`) — same "3 slots" shape as this project's `diskLines`. `data['disks']`
is populated in `pm_auto/pm_auto/addons/system.py` from
`sf_rpi_status.get_disks_info()`.

`sf_rpi_status` (fetched at
[`06990ab`](https://github.com/sunfounder/sf_rpi_status/commit/06990abf5e72589257c74fe450012e02c253f59f),
also **GPLv2**, same reference-only caveat) is where the actual dedup logic
lives, in `sf_rpi_status/status.py`:

- `get_disks()` enumerates **physical disks only**, using `pyudev`:
  ```python
  for device in context.list_devices(subsystem='block', DEVTYPE='disk'):
      device_node = device.device_node
      if 'ram' not in device_node and 'loop' not in device_node:
          all_disks.add(device_node)
  ```
  `DEVTYPE='disk'` is udev's own partition/disk classifier (backed by the
  same kernel `part_type` distinction from §1) — partitions are filtered out
  at enumeration time, so the rest of the pipeline never has to deduplicate
  partition rows after the fact; there's simply one dict entry per physical
  disk from the start.
- `get_disks_info()` then, per physical disk, **sums usage across that
  disk's partitions**: it shells out to `lsblk -b -J -e 7 -o
  NAME,MOUNTPOINT,FSSIZE,FSUSED`, flattens the tree, and for each disk name
  (e.g. `mmcblk0`) matches every `lsblk` entry whose name equals or starts
  with it (`mmcblk0` itself or `mmcblk0p1`, `mmcblk0p2`, …) that has a
  mountpoint, adding each one's `fssize`/`fsused` into a running total/used
  for that one disk row. Percent is then recomputed from the combined
  total/used, not copied from either partition individually.
- `get_disk_type()` classifies by name prefix — `nvme*` → `nvme`, `mmcblk*`
  → `sd`, `md*` → `raid`, `sd*` → `usb` or `hd` (disambiguated by resolving
  `/sys/class/block/<disk>/device/../../../bus` and checking if it resolves
  to `usb`) — the same shape of prefix-based classification this project's
  `classifyDiskType` already does, just with one extra sysfs check to split
  USB from other SCSI/SATA disks (this project's `classifyDiskType` doesn't
  make that USB-vs-SATA distinction yet; out of scope for this ticket, but
  noted since it uses the same enumeration groundwork).

**Answer to the research question**: yes, `pm_auto`'s reference
implementation solves the same-physical-device dedup problem, and it does so
by enumerating physical disks first (udev `DEVTYPE=disk`) rather than
enumerating mounts and grouping backward, then summing each disk's
partitions' usage into one row.

## 3. `l-you/pironman5-go` — sidesteps the problem, doesn't solve it

Fetched `l-you/pironman5-go` at
[`f88cdc6`](https://github.com/l-you/pironman5-go/commit/f88cdc63f1f1b5d0dc12de591af7f5826b14cbb9)
(branch `main`). GitHub reports this repo as **GPL-2.0** as well (a
`LICENCE` file at the repo root, `gpl-2.0` per the GitHub API) — same
reference-only caveat as above; PER-20's research into this repo covered its
OLED icon/font code but didn't need to check its license since that research
was about pixel dimensions, not disk logic, so it's restated here.

It has a disk page (`config.OLEDPageDisk`, rendered by `renderDisks` in
`internal/oled/oled.go`), fed by `internal/status/status.go`'s `readDisks()`:

```go
func readDisks(ctx context.Context) []Disk {
	partitions, err := godisk.PartitionsWithContext(ctx, false) // gopsutil
	...
	seen := make(map[string]struct{})
	for _, partition := range partitions {
		if _, ok := seen[partition.Mountpoint]; ok {
			continue
		}
		seen[partition.Mountpoint] = struct{}{}
		...
		out = append(out, Disk{Name: filepath.Base(partition.Device), ...})
	}
	return out
}
```

This dedupes only by **mountpoint** (via `gopsutil/v4/disk`, a third-party
dependency this project doesn't otherwise use) — it has exactly the same
same-physical-device blind spot PER-25 has: `mmcblk0p1` and `mmcblk0p2` are
different mountpoints, so both survive as separate `Disk` entries here too.

It never groups by physical device. Instead, `oled.go`'s `selectedDisks` /
`aggregateDisks` sidestep the multi-row problem entirely:

```go
func selectedDisks(snap status.Snapshot, cfg config.System) []status.Disk {
	target := strings.TrimSpace(cfg.OLEDDisk)
	if target == "" || strings.EqualFold(target, "total") {
		return aggregateDisks(snap.Disks) // sum ALL disks into one "total" row
	}
	...  // else: match exactly one configured disk name/mountpoint
}
```

`aggregateDisks` sums every disk's used/total into a single synthetic
`{Name: "total", ...}` row (config validation requires `OLEDDisk` be
non-empty, so a fresh install's default is presumably `"total"`). The only
other path is the user pinning `oled_disk` to one specific device or
mountpoint name in config, which returns exactly that one disk. The
multi-row branch in `renderDisks` (`for i, disk := range disks { if i >= 2
{break} }`) is effectively dead code for the common cases — `aggregateDisks`
never returns more than one row, and the exact-match branch returns at most
one match per configured target.

**Answer to the research question**: `pironman5-go` does not implement
per-physical-device grouping either. It avoids the multi-row problem by
either summing literally everything into one lump row, or requiring the user
to hand-pick a single disk/mountpoint in config — a materially different
(and less informative on multi-disk setups) design than `pm_auto`'s
per-physical-disk rows.

## Recommendation

**Naming-based grouping**, using the exact inverse of the kernel's own
partition-naming rule confirmed in §1: given a device name, strip a trailing
`p<digits>` suffix if the remainder ends in a digit, otherwise strip a
trailing run of digits — either way, what's left is the parent device name.
Group `disksFromMounts`' rows by that computed parent name instead of by raw
device name, and **sum used/total bytes across a group's partitions**
(matching `sf_rpi_status.get_disks_info()`'s behavior in §2), recomputing
`Percent` from the summed totals rather than reusing either partition's own
percentage.

Rationale, weighing this against the sysfs-based alternative from §1:

- **This is not a heuristic** — it's the literal inverse of the naming rule
  `add_partition()` uses to create the name in the first place. It isn't
  "probably right for known Pi hardware," it's "correct for how the Linux
  block layer names every partition of every disk," matching the standing
  instruction not to invent a naming convention when the authoritative rule
  is on hand.
- **Fits the existing code exactly**: `classifyDiskType` in
  `internal/sysstats/sysstats.go` already classifies disks by string-parsing
  the device name (`strings.HasPrefix(device, "/dev/mmcblk")`, etc.), with no
  sysfs or syscall access at all. Grouping by a parent name derived the same
  way (string parsing, no I/O) sits right next to that function, reuses the
  data `disksFromMounts` already has in hand from `/proc/mounts`, and adds
  zero new syscalls to a function that today does exactly one read per data
  source (`os.ReadFile`) — no new `/sys` reads, no `os.Readlink`, no new
  error path for "what if this device has no sysfs entry" (relevant should
  this ever run inside a container/chroot test environment where
  `/sys/class/block` may not reflect the host's real device tree at all).
- **The sysfs `/sys/class/block/<dev>/partition` + parent-symlink approach
  from §1 is more robust in the abstract** — it reads the kernel's own
  record instead of re-deriving it, so it can't be wrong about a naming
  scheme nobody has thought of yet. But that robustness answers a threat
  this ticket doesn't have evidence for: `classifyDiskType` already commits
  this codebase to a closed, known set of device families (`nvme`, `mmcblk`,
  `md`, `sd`, else `hd`), and every one of those families follows the exact
  digit-suffix rule in §1. Per this repo's CLAUDE.md and ponytail guidance
  (prefer the simplest correct approach, treat new I/O paths/dependencies as
  unjustified without proof), the extra `/sys` reads buy correctness on a
  case this project isn't going to hit.
- **Sum, not one representative partition**: for a Pi's SD card, `/` and
  `/boot/firmware` are both real capacity/usage on the same physical medium;
  showing only `/`'s numbers would silently drop `/boot/firmware`'s used
  bytes from the total and understate how full the card actually is. Summing
  also matches `sf_rpi_status`'s behavior exactly, so this project's grouped
  row means the same thing a Pironman 5 owner coming from the Python daemon
  would expect it to mean.

**Known, explicitly out-of-scope edge case**: this naming rule (and
`classifyDiskType` today) only reasons about `/dev/<name><digits>` /
`/dev/<name>p<digits>` device paths. It does not cover LVM/device-mapper
paths (`/dev/mapper/*`, `/dev/dm-N`) or `/dev/disk/by-*` symlinks, which don't
carry a parent-disk relationship in their name at all. `disksFromMounts`
already only looks at `/dev/`-prefixed entries from `/proc/mounts`, so this
isn't a regression this ticket introduces — flagging it rather than silently
leaving it unhandled, since a future multi-disk RAID/LVM setup would need a
different (sysfs-based) grouping strategy than the one recommended here.
