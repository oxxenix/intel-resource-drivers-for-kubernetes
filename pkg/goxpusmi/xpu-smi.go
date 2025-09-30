/*
 * Copyright (c) 2025, Intel Corporation.  All Rights Reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

// Package goxpusmi is a Go package, serves as a bridge to work
// with xpu-smi C library. It allows access to native Intel GPU device
// commands and information.
package goxpusmi

/*
#cgo LDFLAGS: "/usr/lib/x86_64-linux-gnu/libxpum.so" -ldl -Wl,--unresolved-symbols=ignore-all
#include "xpum_api.h"
#include <stdlib.h>
*/
import "C"

import (
	"errors"
	"fmt"
	"strconv"
	"unsafe"

	"k8s.io/klog/v2"

	"github.com/intel/intel-resource-drivers-for-kubernetes/pkg/helpers"
)

type XPUSMIDeviceDetails struct {
	UUID         string
	DeviceId     int
	DeviceName   string
	PCIDeviceId  string
	PCIAddress   string
	VendorName   string
	FunctionType int
	DRMDevice    string
	MemoryMiB    uint64
}

var (
	ErrNotIntialized      = errors.New("xpu-smi is not initialized")
	ErrInvalidArgument    = errors.New("invalid argument")
	ErrNotSupported       = errors.New("not supported")
	ErrAlreadyInitialized = errors.New("xpu-smi is already initialized")
	ErrNotFound           = errors.New("not found")
	ErrInsufficientSize   = errors.New("insufficient size")
	ErrDriverNotLoaded    = errors.New("driver is not loaded")
	ErrMemoryError        = errors.New("memory error")
	ErrNoData             = errors.New("no data")
	ErrUnknownError       = errors.New("unknown error")
	propertyNames         = map[string]C.xpum_device_property_name_t{
		"Driver Version":                  C.XPUM_DEVICE_PROPERTY_DRIVER_VERSION,
		"GFX Firmware Version":            C.XPUM_DEVICE_PROPERTY_GFX_FIRMWARE_VERSION,
		"GFX Firmware Name":               C.XPUM_DEVICE_PROPERTY_GFX_FIRMWARE_NAME,
		"GFX Data Firmware Name":          C.XPUM_DEVICE_PROPERTY_GFX_DATA_FIRMWARE_NAME,
		"GFX Data Firmware Version":       C.XPUM_DEVICE_PROPERTY_GFX_DATA_FIRMWARE_VERSION,
		"AMC Firmware Name":               C.XPUM_DEVICE_PROPERTY_AMC_FIRMWARE_NAME,
		"AMC Firmware Version":            C.XPUM_DEVICE_PROPERTY_AMC_FIRMWARE_VERSION,
		"Memory Max Allocatable Size (B)": C.XPUM_DEVICE_PROPERTY_MAX_MEM_ALLOC_SIZE_BYTE,
		"Memory Physical Size (B)":        C.XPUM_DEVICE_PROPERTY_MEMORY_PHYSICAL_SIZE_BYTE,
		"Memory Free (B)":                 C.XPUM_DEVICE_PROPERTY_MEMORY_FREE_SIZE_BYTE,
		"Memory Bus Width (bit)":          C.XPUM_DEVICE_PROPERTY_MEMORY_BUS_WIDTH,
		"Number of Tiles":                 C.XPUM_DEVICE_PROPERTY_NUMBER_OF_TILES,
		"Number of EUs":                   C.XPUM_DEVICE_PROPERTY_NUMBER_OF_EUS,
	}
	// Used as Taint keys => need to conform to their format (no spaces etc).
	healthTypes = map[string]C.xpum_health_type_t{
		"CoreThermal":   C.XPUM_HEALTH_CORE_THERMAL,
		"MemoryThermal": C.XPUM_HEALTH_MEMORY_THERMAL,
		"Power":         C.XPUM_HEALTH_POWER,
		"Memory":        C.XPUM_HEALTH_MEMORY,
		"FabricPort":    C.XPUM_HEALTH_FABRIC_PORT,
		"Frequency":     C.XPUM_HEALTH_FREQUENCY,
	}
	healthStatuses = map[C.xpum_health_status_t]string{
		C.XPUM_HEALTH_STATUS_UNKNOWN:  "Unknown",
		C.XPUM_HEALTH_STATUS_OK:       "OK",
		C.XPUM_HEALTH_STATUS_WARNING:  "Warning",
		C.XPUM_HEALTH_STATUS_CRITICAL: "Critical",
	}
	// deviceHealthCache caches last known health status per device per health type.
	// Outer key: device id, inner key: health type, value: last status.
	deviceHealthCache = make(map[C.xpum_device_id_t]map[C.xpum_health_type_t]C.xpum_health_status_t)
)

func errorString(ret C.xpum_result_t) error {
	switch ret {
	case C.XPUM_OK:
		return nil
	case C.XPUM_NOT_INITIALIZED:
		return ErrNotIntialized
	case C.XPUM_GENERIC_ERROR:
		return ErrUnknownError
	}

	return fmt.Errorf("invalid libxpum error return code %d", ret)
}

// Initialize initializes the libxpum.
func Initialize() error {
	return errorString(C.xpumInit())
}

// Shutdown shuts down the libxpum.
func Shutdown() error {
	return errorString(C.xpumShutdown())
}

// Discover returns a PCIAddress:XPUSMIDeviceDetails map of Intel GPU devices discovered
// by libxpum in the system.
func Discover(verbose bool) (map[string]XPUSMIDeviceDetails, error) {
	deviceDetails := map[string]XPUSMIDeviceDetails{}
	var devices [C.XPUM_MAX_NUM_DEVICES]C.xpum_device_basic_info
	count := C.int(C.XPUM_MAX_NUM_DEVICES)

	err := C.xpumGetDeviceList(&devices[0], &count)
	if err != C.XPUM_OK {
		fmt.Printf("xpumGetDeviceList err: %v\n", err)
		return nil, errorString(err)
	}

	for i := 0; i < int(count); i++ {
		device := devices[i]
		newDeviceDetails := XPUSMIDeviceDetails{
			UUID:        C.GoString(&device.uuid[0]),
			DeviceId:    int(device.deviceId),
			DeviceName:  C.GoString(&device.deviceName[0]),
			PCIDeviceId: C.GoString(&device.PCIDeviceId[0]),
			PCIAddress:  C.GoString(&device.PCIBDFAddress[0]),
			VendorName:  C.GoString(&device.VendorName[0]),
			// TODO: handle functionType properly
			FunctionType: int(device.functionType),
			DRMDevice:    C.GoString(&device.drmDevice[0]),
		}
		// HACK:
		// xpu-smi v1.3.1 could omit leading zeros in PCIDeviceId, e.g. 0xbda instead of 0x0bda.
		if len(newDeviceDetails.PCIDeviceId) < 6 {
			fmt.Printf("WARNING: PCIDeviceId is shorter than expected: '%s'. Prepending zeros.\n", newDeviceDetails.PCIDeviceId)
			newDeviceDetails.PCIDeviceId = fmt.Sprintf("0x%04s", newDeviceDetails.PCIDeviceId[2:])
		}

		if verbose {
			fmt.Printf("Device %+v\n", newDeviceDetails)
		}

		GetAndPrintDeviceProperties(device.deviceId, &newDeviceDetails, verbose)

		deviceDetails[newDeviceDetails.PCIAddress] = newDeviceDetails
	}

	return deviceDetails, errorString(err)
}

// GetDeviceDetailsByUUID populates memory in deviceDetails and prints all properties of a device by its UUID.
func GetAndPrintDeviceProperties(deviceId C.xpum_device_id_t, deviceDetails *XPUSMIDeviceDetails, verbose bool) {
	var properties C.xpum_device_properties_t
	err := C.xpumGetDeviceProperties(deviceId, &properties)
	if err != C.XPUM_OK {
		fmt.Printf("Failed to get device properties: %v\n", err)
		return
	}

	// Iterate over the properties and print them.
	for propertyName, propertyId := range propertyNames {
		if C.int(propertyId) >= properties.propertyLen {
			fmt.Printf("ERROR: Property %s not found in device properties. SKIPPING\n", propertyName)
			continue
		}
		propertyItem := properties.properties[propertyId]
		propertyStr := C.GoString(&propertyItem.value[0])
		if verbose {
			fmt.Printf("\t\t%s: %s\n", propertyName, propertyStr)
		}
		if C.XPUM_DEVICE_PROPERTY_MEMORY_PHYSICAL_SIZE_BYTE == propertyId {
			propertyUint, err := strconv.ParseUint(propertyStr, 10, 64)
			if err != nil {
				fmt.Printf("Failed to parse memory amount: %v\n", err)
				continue
			}
			deviceDetails.MemoryMiB = propertyUint / 1024 / 1024
		}
	}
}

// HealthCheck performs a health check using libxpum library, updates an internal per-device health cache,
// and returns only changed health type statuses since the previous call (map[deviceUID]map[healthType]status).
// An empty map means no changes.
func HealthCheck(devices map[string]XPUSMIDeviceDetails) (updates map[string]map[string]string) {
	updates = make(map[string]map[string]string)
	for _, device := range devices {
		// If this device is seen for the first time, initialize baseline health
		// statuses to OK for every health type. This prevents emitting a wave of
		// "UNKNOWN -> OK" transitions on startup when everything is healthy.
		devId := C.xpum_device_id_t(device.DeviceId)
		if _, exists := deviceHealthCache[devId]; !exists {
			baseline := make(map[C.xpum_health_type_t]C.xpum_health_status_t, len(healthTypes))
			for _, healthType := range healthTypes {
				baseline[healthType] = C.XPUM_HEALTH_STATUS_OK
			}
			deviceHealthCache[devId] = baseline
		}
		for healthTypeName, healthType := range healthTypes {
			var healthData C.xpum_health_data_t
			ret := C.xpumGetHealth(C.xpum_device_id_t(device.DeviceId), C.xpum_health_type_t(healthType), &healthData)
			if ret != C.XPUM_OK {
				fmt.Printf("Failed to get health for device %d, health type %d\n", device.DeviceId, healthType)
				continue
			}
			prevStatus := deviceHealthCache[healthData.deviceId][healthType]
			currStatus := healthData.status

			if prevStatus == currStatus {
				// Health status did not change; skip the following.
				continue
			}

			// Update the changed health status.
			deviceHealthCache[healthData.deviceId][healthType] = currStatus
			// Ensure updates entry for this device UUID exists.
			deviceUID := helpers.DeviceUIDFromPCIinfo(device.PCIAddress, device.PCIDeviceId)
			if _, ok := updates[deviceUID]; !ok {
				updates[deviceUID] = make(map[string]string)
			}
			updates[deviceUID][healthTypeName] = healthStatuses[currStatus]
			klog.V(3).Infof("Device %d health change. Type='%s' prev='%s' curr='%s' description='%s'",
				device.DeviceId, healthTypeName, healthStatuses[prevStatus], healthStatuses[currStatus], C.GoString(&healthData.description[0]))
		}
	}
	return updates
}

// SetHealthConfig sets health configuration for all devices.
func SetHealthConfig(devices map[string]XPUSMIDeviceDetails, healthConfigType string, healthConfigValue int) {
	for _, device := range devices {
		devId := device.DeviceId
		// nolint:gocritic // false positive: no duplicated sub-expression
		result := C.xpumSetHealthConfig(C.xpum_device_id_t(devId), HealthConfigTypeFromString(healthConfigType), unsafe.Pointer(&healthConfigValue))
		if result != C.XPUM_OK {
			panic(fmt.Sprintf("Set '%s' health threshold for device %d: value='%d' failed: %v", healthConfigType, devId, healthConfigValue, errorString(result)))
		}
		klog.V(3).Infof("Set health '%s' threshold for device %d: value='%d', result=<%d>", healthConfigType, devId, healthConfigValue, result)
	}
}

func HealthConfigTypeFromString(healthConfigType string) C.xpum_health_config_type_t {
	switch healthConfigType {
	case "CoreThermalLimit":
		return C.XPUM_HEALTH_CORE_THERMAL_LIMIT
	case "MemoryThermalLimit":
		return C.XPUM_HEALTH_MEMORY_THERMAL_LIMIT
	case "PowerLimit":
		return C.XPUM_HEALTH_POWER_LIMIT
	}
	panic("invalid health config type: " + healthConfigType)
}
