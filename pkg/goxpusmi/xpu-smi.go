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

	return fmt.Errorf("invalid HLML error return code %d", ret)
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
		if verbose {
			fmt.Printf("Device %d\n", i)
			fmt.Printf("\tuuid: %v\n", C.GoString(&device.uuid[0]))
			fmt.Printf("\tdeviceId: %v\n", device.deviceId)
			fmt.Printf("\tdeviceName: %v\n", C.GoString(&device.deviceName[0]))
			fmt.Printf("\tPCIDeviceId: %v\n", C.GoString(&device.PCIDeviceId[0]))
			fmt.Printf("\tPCIBDFAddress: %v\n", C.GoString(&device.PCIBDFAddress[0]))
			fmt.Printf("\tVendorName: %v\n", C.GoString(&device.VendorName[0]))
			fmt.Printf("\tfunctionType: %v\n", device.functionType)
			fmt.Printf("\tdrmDevice: %v\n", C.GoString(&device.drmDevice[0]))
		}

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

	// iterate over the properties and print them
	for propertyName, propertyId := range propertyNames {
		if C.int(propertyId) >= properties.propertyLen {
			fmt.Printf("ERROR: Property %s not found in device properties. SKIPPING\n", propertyName)
			continue
		}
		propertyItem := properties.properties[propertyId]
		if verbose {
			fmt.Printf("\t\t%s: %s\n", propertyName, C.GoString(&propertyItem.value[0]))
		}
		if C.XPUM_DEVICE_PROPERTY_MEMORY_PHYSICAL_SIZE_BYTE == propertyId {
			deviceDetails.MemoryMiB = uint64(propertyItem.value[0]) / 1024 / 1024
		}
	}
}
