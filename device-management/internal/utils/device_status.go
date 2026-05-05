package utils

import (
	"time"

	v1 "github.com/artpark-hub/taksa-platform/device-management/api/devicemgmt/v1"
)

// DeriveEffectiveDeviceStatus derives the "effective" device status used for list/health views.
// Rules:
// - PENDING stays PENDING until first login flips it
// - SUSPENDED/DECOMMISSIONED are terminal admin statuses
// - ACTIVE/INACTIVE are derived from last_seen within the provided activeWindow
func DeriveEffectiveDeviceStatus(device *v1.Device, activeWindow time.Duration) v1.DeviceStatus {
	if device == nil {
		return v1.DeviceStatus_DEVICE_STATUS_UNSPECIFIED
	}

	switch device.Status {
	case v1.DeviceStatus_SUSPENDED, v1.DeviceStatus_DECOMMISSIONED:
		return device.Status
	case v1.DeviceStatus_PENDING:
		return v1.DeviceStatus_PENDING
	default:
		// Only derive for ACTIVE/INACTIVE/UNSPECIFIED. Preserve any other admin states.
		if device.Status != v1.DeviceStatus_ACTIVE &&
			device.Status != v1.DeviceStatus_INACTIVE &&
			device.Status != v1.DeviceStatus_DEVICE_STATUS_UNSPECIFIED {
			return device.Status
		}
	}

	if device.LastSeen != nil {
		if time.Since(device.LastSeen.AsTime()) <= activeWindow {
			return v1.DeviceStatus_ACTIVE
		}
		return v1.DeviceStatus_INACTIVE
	}
	return v1.DeviceStatus_INACTIVE
}

