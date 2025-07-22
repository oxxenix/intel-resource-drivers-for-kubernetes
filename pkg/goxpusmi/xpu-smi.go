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

var (
	ErrNotIntialized      = errors.New("xpu-smi is not initialized")
	ErrInvalidArgument    = errors.New("invalid argument")
	ErrNotSupported       = errors.New("not supported")
	ErrAlreadyInitialized = errors.New("xpu-smi is already initialized")
	ErrNotFound           = errors.New("not found")
	ErrInsufficientSize   = errors.New("insufficient size")
	ErrDriverNotLoaded    = errors.New("driver not loaded")
	ErrMemoryError        = errors.New("memory error")
	ErrNoData             = errors.New("no data")
	ErrUnknownError       = errors.New("unknown error")
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

// Initialize initializes the xpusmi library
func Initialize() error {
	return errorString(C.xpumInit())
}

// Shutdown shutdowns the HLML library
func Shutdown() error {
	return errorString(C.xpumShutdown())
}

// DeviceCount gets number of Habana devices in the system
func DeviceCount() (uint, error) {
	var devices [C.XPUM_MAX_NUM_DEVICES]C.xpum_device_basic_info
	count := C.int(C.XPUM_MAX_NUM_DEVICES)

	err := C.xpumGetDeviceList(&devices[0], &count)
	fmt.Printf("xpumGetDeviceList returned %d devices, err: %v\n", count, err)

	for i := 0; i < int(count); i++ {
		fmt.Printf("Device %d\n", i)
		fmt.Printf("\tuuid: %v\n", C.GoString(&devices[i].uuid[0]))
		fmt.Printf("\tdeviceId: %v\n", devices[i].deviceId)
		fmt.Printf("\tdeviceName: %v\n", C.GoString(&devices[i].deviceName[0]))
		fmt.Printf("\tPCIDeviceId: %v\n", C.GoString(&devices[i].PCIDeviceId[0]))
		fmt.Printf("\tPCIBDFAddress: %v\n", C.GoString(&devices[i].PCIBDFAddress[0]))
		fmt.Printf("\tVendorName: %v\n", C.GoString(&devices[i].VendorName[0]))
		fmt.Printf("\tfunctionType: %v\n", devices[i].functionType)
		fmt.Printf("\tdrmDevice: %v\n", C.GoString(&devices[i].drmDevice[0]))
	}

	return uint(count), errorString(err)
}
