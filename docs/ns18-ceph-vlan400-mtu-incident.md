# NS18 Ceph VLAN 400 MTU Investigation

Date: June 6, 2026

## Summary

Silo playback startup on `silo-new` / CT135 was slow because cold reads from CephFS on host `ns18` were extremely slow or timing out. The same read pattern on `ns11` was fast.

The investigation found that `ns18` had an inconsistent jumbo-frame path on VLAN 400, the Ceph storage network. Small packets worked, but larger DF packets failed to some Ceph peers. This caused CephFS reads to stall depending on which Ceph peers/OSDs were involved.

A temporary mitigation was applied on `ns18`: force the Ceph VLAN interface `vmbr0.400` to MTU 1500. After that, host and container Ceph reads dropped from 35-second timeouts to under 1 second.

This is a mitigation, not the final network fix. The desired final state is for `ns18` to pass jumbo frames on VLAN 400 end-to-end like `ns11`.

## Affected Systems

- Host: `ns18.wave-ninja.eu`
- Container: CT135, `silo-new`
- Application: Silo native debug service
- Storage path: CephFS mounted under `/mnt/sharedrives/zd-storage-ceph`
- Network: VLAN 400, Ceph storage network

## Symptoms

Playback startup for transcoded media was slow, often around 9 to 10 seconds or worse.

Application-side playback investigation showed that ffmpeg and app-level restart behavior were no longer the main cause. Hot-cache ffmpeg startup was fast, but cold media reads from CephFS on `ns18` were slow.

Host-level cold read tests from `ns18` showed 64 MB direct reads timing out at 35 seconds. The same style of test on `ns11` completed in about 0.8 to 1.0 seconds.

## Important Ruling-Out

The issue was not caused by:

- Silo application code
- ffmpeg transcode command selection
- CT135 / LXC bind mount behavior
- Ceph client key/caps alone

Evidence for the key/caps point: `ns18` was temporarily mounted using the same Ceph client key used on `ns11`, and reads still timed out.

Stopping CT135 also did not resolve the host-level read problem, so CT135 workload was not the cause.

## Host Comparison Findings

`ns11` and `ns18` are now similar in several important ways:

- Same OS family: Debian 13 / trixie
- Same Proxmox kernel: `7.0.2-6-pve`
- Same CephFS monitor addresses
- Similar CephFS mount options
- Same general VLAN 400 storage network

They were not identical before the June 6 package alignment:

- `ns11` had Ceph packages at `19.2.3-pve4`.
- `ns18` had Ceph packages at `19.2.3-pve2`.
- `ns11` was using the Proxmox Ceph Squid no-subscription repo.
- `ns18` did not have the Ceph Squid no-subscription source enabled, so `19.2.3-pve4` was not visible as a candidate.
- `ns18` NIC counters showed `rx_missed_errors=992`; `ns11` showed `0`
- Most importantly, `ns18` had an inconsistent jumbo-frame path on VLAN 400

The Ceph package/source mismatch has now been fixed on `ns18`. The jumbo-frame failure remains.

The MTU finding is the strongest match for the observed behavior.

## NS11 vs NS18 Side-by-Side

| Area | NS11 | NS18 | Notes |
| --- | --- | --- | --- |
| Hostname | `ns11.wave-ninja.eu` | `ns18.wave-ninja.eu` | Compared as working vs slow host |
| OS | Debian GNU/Linux 13 / trixie | Debian GNU/Linux 13 / trixie | Same OS family |
| Kernel | `7.0.2-6-pve` | `7.0.2-6-pve` | Same Proxmox kernel |
| Ceph monitors | `10.10.3.58`, `10.10.3.59`, `10.10.3.60` | `10.10.3.58`, `10.10.3.59`, `10.10.3.60` | Same monitor set |
| CephFS mount style | Kernel CephFS mounts under `/mnt/sharedrives/zd-storage-ceph` | Kernel CephFS mounts under `/mnt/sharedrives/zd-storage-ceph` | Same overall mount model |
| CephFS mount options | `rw,noatime`, `ms_mode=secure`, `noshare`, `recover_session=clean`, `rasize=67108864`, `caps_wanted_delay_max=30`, `readdir_max_entries=32768`, `readdir_max_bytes=33554432` | Same effective options | Mount options were effectively aligned |
| Media client key | `c0e33113...` | `a0c4c4f...` | Different as expected |
| Alternate key test | Native key was already fast | `ns18` was tested with `ns11` key and remained slow | Rules out key/caps as the primary cause |
| Ceph packages | `19.2.3-pve4` | `19.2.3-pve4` after June 6 package alignment | Now matches |
| Ceph repo source | Proxmox Ceph Squid no-subscription repo | Proxmox Ceph Squid no-subscription repo after June 6 package alignment | Now matches |
| Active storage NIC | `eno1np0` | `nic2` | Both are `i40e` |
| NIC firmware | `9.56 0x80010165 25.0.4` | `9.56 0x80010159 25.0.4` | Slight firmware/build identifier difference |
| NIC errors | `rx_missed_errors=0` | `rx_missed_errors=992` | Not necessarily causal, but another difference |
| VLAN 400 runtime MTU before mitigation | 9000 | 9000 inherited before mitigation | Both appeared configured for jumbo |
| Jumbo DF ping behavior | Passed 8972-byte DF pings to tested Ceph peers | Failed jumbo DF pings to several Ceph peers | Primary behavioral difference |
| 64 MB direct cold read behavior before mitigation | About `0.84s` to `0.99s` | Timed out at 35s on repeated samples | Matches playback startup symptoms |
| Current mitigation | None needed | `vmbr0.400 mtu 1500` | Makes reads reliable, but not identical config |
| 64 MB direct read behavior after mitigation | Not changed | About `0.76s` to `0.92s` on host, `0.77s` to `0.89s` in CT135 | Confirms MTU workaround fixed read stalls |

Bottom line: `ns11` and `ns18` are no longer configured identically because `ns18` is intentionally forced to MTU 1500 as a workaround. They are now aligned from a reliability/read-performance perspective, but the final infrastructure fix is still to make `ns18` pass jumbo frames on VLAN 400 like `ns11`.

## Ceph Client Version Alignment Completed

On June 6, 2026, `ns18` was updated to use the same Proxmox Ceph Squid no-subscription repository as `ns11`:

```text
Types: deb
URIs: http://download.proxmox.com/debian/ceph-squid
Suites: trixie
Components: no-subscription
Signed-By: /usr/share/keyrings/proxmox-archive-keyring.gpg
```

The following packages now match between `ns11` and `ns18`:

```text
ceph-common              19.2.3-pve4
ceph-fuse                19.2.3-pve4
libcephfs2               19.2.3-pve4
librados2                19.2.3-pve4
libradosstriper1         19.2.3-pve4
librbd1                  19.2.3-pve4
librgw2                  19.2.3-pve4
python3-ceph-argparse    19.2.3-pve4
python3-ceph-common      19.2.3-pve4
python3-cephfs           19.2.3-pve4
python3-rados            19.2.3-pve4
python3-rbd              19.2.3-pve4
python3-rgw              19.2.3-pve4
```

Both hosts now report the same Ceph version:

```text
ceph version 19.2.3 (d74d168b1c80fb01e1a30d5e4ca9a45b12bc145b) squid (stable)
```

After this package alignment, a controlled MTU 9000 test was run again on `ns18`. Jumbo-frame failures remained:

```text
Test file: /root/ns18-post-ceph-pve4-mtu9000-test-20260606-124442.txt
Result: 10 jumbo test failures
```

This confirms the remaining mismatch is not the Ceph userspace package version. The remaining mismatch is the VLAN 400 jumbo-frame path.

## Jumbo Frame Evidence

When `ns18` was temporarily set back to MTU 9000 on `vmbr0.400`, the following DF ping matrix was collected.

Payload sizes:

- `1472` byte payload: standard 1500 MTU test
- `4000` byte payload: mid-size jumbo test
- `8972` byte payload: near-9000 MTU jumbo test

Results from `ns18` with `vmbr0.400` at MTU 9000:

```text
=== 10.10.3.58 ===
payload=1472  pass
payload=4000  pass
payload=8972  pass

=== 10.10.3.59 ===
payload=1472  pass
payload=4000  fail
payload=8972  pass

=== 10.10.3.60 ===
payload=1472  pass
payload=4000  fail
payload=8972  fail

=== 10.10.3.12 ===
payload=1472  pass
payload=4000  fail
payload=8972  fail

=== 10.10.3.18 ===
payload=1472  pass
payload=4000  fail
payload=8972  fail

=== 10.10.3.43 ===
payload=1472  pass
payload=4000  fail
payload=8972  pass
```

By comparison, `ns11` passed 8972-byte DF pings to the same tested Ceph peers.

This pattern shows that:

- `ns18` can send/receive normal 1500-byte traffic on VLAN 400.
- `ns18` can send jumbo traffic to some peers.
- `ns18` cannot send jumbo traffic reliably to all Ceph peers.
- The failure is peer/path-dependent, not a simple local host setting issue.

Proof file on `ns18`:

```text
/root/ns18-ceph-jumbo-failure-proof-20260606-122636.txt
```

## Mitigation Applied

The Ceph VLAN interface on `ns18` was forced to MTU 1500.

Runtime change:

```bash
ip link set vmbr0.400 mtu 1500
```

Persistent config changed in:

```text
/etc/network/interfaces
```

Backup created before the persistent edit:

```text
/etc/network/interfaces.backup-before-ceph-mtu-1500-20260606-122703
```

Current `vmbr0.400` stanza:

```text
auto vmbr0.400
iface vmbr0.400 inet static
    address 10.10.7.213/21
    mtu 1500
    dns-nameservers 10.10.0.1
    dns-search cephstorage.local
#VLAN 400 - Ceph storage network
```

This keeps `ns18` from attempting jumbo packets on the broken path.

## Result After Mitigation

After forcing `vmbr0.400` to MTU 1500, 64 MB direct Ceph reads completed quickly.

Host `ns18` read tests:

```text
elapsed=0.87
elapsed=0.92
elapsed=0.90
elapsed=0.76
elapsed=0.77
```

CT135 / `silo-new` read tests:

```text
elapsed=0.88
elapsed=0.82
elapsed=0.77
elapsed=0.89
elapsed=0.80
```

Before mitigation, comparable `ns18` reads timed out at 35 seconds or took far too long.

## Current Application State

CT135 is running.

The Docker Silo application container is stopped to avoid port conflicts:

```text
silo-server-silo-1 Exited
```

The native debug service is running:

```text
silo-native.service active
```

Listeners:

```text
Silo HTTP:            :8080
Jellyfin compat API:  :8096
```

Smoke checks passed:

```text
http://127.0.0.1:8080/                    200
http://127.0.0.1:8096/System/Info/Public  200
```

## What Changed On NS18

1. `vmbr0.400` MTU changed from inherited jumbo/9000 behavior to explicit MTU 1500.
2. `/etc/network/interfaces` was updated to persist `mtu 1500` for `vmbr0.400`.
3. CT135 was restarted after host-level testing.
4. GNU `time` was installed inside CT135 for read timing tests.
5. Docker Silo app container was stopped.
6. Native debug service `silo-native.service` was created/started to run `/opt/silo-native/silo`.

## Network Team Request

Please investigate VLAN 400 jumbo-frame handling on the `ns18` path end-to-end.

The target final state is:

```text
ns18 should pass 8972-byte DF pings to the same Ceph peers that ns11 can.
```

Areas to verify:

- Switch port connected to `ns18`
- VLAN 400 trunk/native VLAN configuration
- MTU on all relevant switch ports
- MTU on any port-channel, LACP, MLAG, bridge, or intermediate switching path
- Any ACL, storm-control, QoS, or fabric policy that could affect larger frames
- Ceph peer paths for `10.10.3.58`, `10.10.3.59`, `10.10.3.60`, `10.10.3.12`, `10.10.3.18`, and `10.10.3.43`

The key point is that small packets work from `ns18`, but jumbo packets fail inconsistently depending on destination. That is why normal connectivity checks can pass while CephFS cold reads still stall badly.

## Suggested Validation Commands

Run from `ns18` after restoring jumbo MTU for testing:

```bash
ip link set vmbr0.400 mtu 9000

for h in 10.10.3.58 10.10.3.59 10.10.3.60 10.10.3.12 10.10.3.18 10.10.3.43; do
  echo "=== $h ==="
  ping -c 2 -M do -s 1472 -W 1 "$h"
  ping -c 2 -M do -s 4000 -W 1 "$h"
  ping -c 2 -M do -s 8972 -W 1 "$h"
done
```

If testing disrupts performance, return to the current mitigation:

```bash
ip link set vmbr0.400 mtu 1500
```

Expected final result after network remediation:

```text
1472-byte DF pings pass to all tested Ceph peers.
4000-byte DF pings pass to all tested Ceph peers.
8972-byte DF pings pass to all tested Ceph peers.
```
