# LogiGate Hardware Protocol (Validated)

This document contains the exact, empirically validated HID++ payloads for switching Logitech hardware on this machine.

## Devices
- **MX Master 3S M** (PID: B034)
- **ERGO 860B** (PID: B364)

## Protocol: HID++ 2.0 (Long Report)
The following sequence is required for a successful hardware switch.

### Header Bytes
1. `0x11` - Report ID (Long Report)
2. `0x01` - **Device Slot/Index** (Crucial: 0x01 is required, 0x00/Broadcast is ignored)
3. `Idx`  - **Feature Index** (0x0A for Mouse, 0x09 for Keyboard)
4. `0x1E` - **Command ID** (Direct Feature Access)

### Payload Layout
`11 [Slot] [FeatureIdx] 1E [Channel] 00 00 ... (total 20 bytes)`

### Channel Mapping
- **Channel 1:** `0x00`
- **Channel 2:** `0x01`
- **Channel 3:** `0x02`

## Validated Test Commands
These commands were tested and physically switched the hardware on this machine:

**Switch Mouse to Channel 2:**
`hidapitester --vidpid 046D:B034 --usage 0x0202 --usagePage 0xFF43 --open --length 20 --send-output 0x11,0x01,0x0A,0x1E,0x01,0x00,0x00,0x00,0x00,0x00,0x00,0x00,0x00,0x00,0x00,0x00,0x00,0x00,0x00,0x00`

**Switch Keyboard to Channel 2:**
`hidapitester --vidpid 046D:B364 --usage 0x0202 --usagePage 0xFF43 --open --length 20 --send-output 0x11,0x01,0x09,0x1E,0x01,0x00,0x00,0x00,0x00,0x00,0x00,0x00,0x00,0x00,0x00,0x00,0x00,0x00,0x00,0x00`
