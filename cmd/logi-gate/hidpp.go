package main

// HID++ 2.0/3.0 Protocol Constants
const (
	LogitechVendorID = 0x046D
	ReportIDLong     = 0x11
	DeviceID         = 0xFF 
)

// Root Commands
const (
	FeatureRoot           = 0x0000
	CommandGetFeatureIdx  = 0x00
	FeatureEasySwitch     = 0x1E00
)

// Easy-Switch Commands
const (
	CommandSwitchDevice   = 0x1E // Note: On many devices the Feature Index is used as the base
)
