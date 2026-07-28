# LogiGate Hardware Protocol (Validated)

This document contains the exact, empirically validated HID++ payloads for switching Logitech hardware on this machine.

## Feature-index resolution (dynamic — no hardcoded table)

The switch targets the **ChangeHost** feature, HID++ 2.0 feature id **`0x1814`**.
Each device assigns that feature its own *feature index*, which varies per model
and per unit — so LogiGate resolves it dynamically at scan time instead of
hardcoding a PID→index table.

**Resolution query** (root feature `getFeature(0x1814)`):
`11 FF 00 00 18 14 00 ... (20 bytes)` → reply `11 FF 00 00 <IDX> ...`, byte 4 = index.

The reply MUST be validated to echo the request header `11 FF 00 00` before
trusting byte 4 — the HID input pipe returns stale/unrelated frames right after
open, and trusting the first frame silently dropped or mis-indexed devices.

> **Do NOT probe `0x1E00`.** It returns the index of a *different* feature
> (observed 0x1B on the MX Master, 0x1A on the K860). A switch sent to that
> index silently no-ops — this was the "channel 3, only the mouse moves" bug.

Empirically validated on this machine (via the `0x1814` query):
- **MX Master 3S** (PID: B034) → ChangeHost index **`0x0A`**
- **ERGO K860** (PID: B359) → ChangeHost index **`0x08`**

## Protocol: HID++ 2.0 (Long Report)
The following sequence is required for a successful hardware switch.

### Header Bytes
1. `0x11` - Report ID (Long Report)
2. `0x01` - **Device Slot/Index** (Crucial: 0x01 is required, 0x00/Broadcast is ignored)
3. `Idx`  - **Feature Index** (the ChangeHost index resolved dynamically above)
4. `0x1E` - **Command ID** (Direct Feature Access)

### Payload Layout
`11 [Slot] [FeatureIdx] 1E [Channel] 00 00 ... (total 20 bytes)`

### Channel Mapping
- **Channel 1:** `0x00`
- **Channel 2:** `0x01`
- **Channel 3:** `0x02`

## Validated Test Commands
These commands were tested and physically switched the hardware on this machine
(using the dynamically-resolved ChangeHost indices above):

**Switch Mouse to Channel 2** (index 0x0A):
`hidapitester --vidpid 046D:B034 --usage 0x0202 --usagePage 0xFF43 --open --length 20 --send-output 0x11,0x01,0x0A,0x1E,0x01,0x00,0x00,0x00,0x00,0x00,0x00,0x00,0x00,0x00,0x00,0x00,0x00,0x00,0x00,0x00`

**Switch Keyboard to Channel 2** (PID B359, index 0x08):
`hidapitester --vidpid 046D:B359 --usage 0x0202 --usagePage 0xFF43 --open --length 20 --send-output 0x11,0x01,0x08,0x1E,0x01,0x00,0x00,0x00,0x00,0x00,0x00,0x00,0x00,0x00,0x00,0x00,0x00,0x00,0x00,0x00`
