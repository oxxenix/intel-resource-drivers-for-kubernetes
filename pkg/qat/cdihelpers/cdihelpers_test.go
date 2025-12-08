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

package cdihelpers

import (
	"sort"
	"testing"

	cdiapi "tags.cncf.io/container-device-interface/pkg/cdi"
	cdispecs "tags.cncf.io/container-device-interface/specs-go"

	"github.com/intel/intel-resource-drivers-for-kubernetes/pkg/fakesysfs"
	testhelpers "github.com/intel/intel-resource-drivers-for-kubernetes/pkg/plugintesthelpers"
	"github.com/intel/intel-resource-drivers-for-kubernetes/pkg/qat/device"
)

const DriverName = "qat"

// TestSyncDevices acts as an orchestrator. It spawns a new process for each test case.
// This ensures that global state (like cached SYSFS_ROOT in the device package) is fresh for every case.
func TestSyncDevices(t *testing.T) {

	testcases := []struct {
		Name          string
		ExistingSpecs []*cdispecs.Spec
		SysfsDevices  fakesysfs.QATDevices
		ExpectedUIDs  []string
	}{
		{
			Name:          "Add 2 VFs to empty spec",
			ExistingSpecs: []*cdispecs.Spec{},
			SysfsDevices: fakesysfs.QATDevices{
				{
					Device:   "0000:4b:00.0",
					State:    "up",
					NumVFs:   2,
					TotalVFs: 2,
				},
			},
			ExpectedUIDs: []string{"qatvf-0000-4b-00-1", "qatvf-0000-4b-00-2"},
		},
		{
			Name: "Replace removed device with 2 VFs",
			ExistingSpecs: []*cdispecs.Spec{
				{
					Kind:    device.CDIKind,
					Version: "0.5.0",
					Devices: []cdispecs.Device{
						{
							Name: "removed-device",
							ContainerEdits: cdispecs.ContainerEdits{
								DeviceNodes: []*cdispecs.DeviceNode{
									{Path: "/dev/null"},
								},
							},
						},
					},
				},
			},
			SysfsDevices: fakesysfs.QATDevices{
				{
					Device:   "0000:4b:00.0",
					State:    "up",
					NumVFs:   2,
					TotalVFs: 2,
				},
			},
			ExpectedUIDs: []string{"qatvf-0000-4b-00-1", "qatvf-0000-4b-00-2"},
		},
		{
			Name: "Delete (only) device",
			ExistingSpecs: []*cdispecs.Spec{
				{
					Kind:    device.CDIKind,
					Version: "0.5.0",
					Devices: []cdispecs.Device{
						{
							Name: "qatvf-0000-4b-00-1",
							ContainerEdits: cdispecs.ContainerEdits{
								DeviceNodes: []*cdispecs.DeviceNode{
									{Path: "/dev/vfio/1"},
								},
							},
						},
					},
				},
			},
			SysfsDevices: fakesysfs.QATDevices{}, // Empty sysfs
			ExpectedUIDs: []string{},
		},
		{
			Name: "Delete 1 of the 2 devices",
			ExistingSpecs: []*cdispecs.Spec{
				{
					Kind:    device.CDIKind,
					Version: "0.5.0",
					Devices: []cdispecs.Device{
						{
							Name: "qatvf-0000-00-00-1",
							ContainerEdits: cdispecs.ContainerEdits{
								DeviceNodes: []*cdispecs.DeviceNode{
									{Path: "/dev/vfio/1"},
								},
							},
						},
						{
							Name: "qatvf-0000-00-00-2",
							ContainerEdits: cdispecs.ContainerEdits{
								DeviceNodes: []*cdispecs.DeviceNode{
									{Path: "/dev/vfio/2"},
								},
							},
						},
					},
				},
			},
			SysfsDevices: fakesysfs.QATDevices{
				{
					Device:   "0000:00:00.0",
					State:    "up",
					NumVFs:   1,
					TotalVFs: 1,
				},
			},
			ExpectedUIDs: []string{"qatvf-0000-00-00-1"},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.Name, func(t *testing.T) {
			testDirs, err := testhelpers.NewTestDirs("qat.intel.com")
			if err != nil {
				t.Errorf("%v: setup error: %v", tc.Name, err)
				return
			}
			defer testhelpers.CleanupTest(t, tc.Name, testDirs.TestRoot)

			t.Setenv("SYSFS_ROOT", testDirs.SysfsRoot)
			defer device.ClearSysfsRoot()

			cache, err := cdiapi.NewCache(cdiapi.WithSpecDirs(testDirs.CdiRoot))
			if err != nil {
				t.Fatalf("Failed to create cache: %v", err)
			}

			specName := cdiapi.GenerateSpecName(device.CDIVendor, device.CDIClass)
			for _, spec := range tc.ExistingSpecs {
				if err := cache.WriteSpec(spec, specName); err != nil {
					t.Fatalf("Failed to write spec: %v", err)
				}
			}
			testhelpers.CDICacheDelay()

			if err := fakesysfs.FakeSysFsQATContents(testDirs.SysfsRoot, tc.SysfsDevices); err != nil {
				t.Errorf("setup error: could not create fake sysfs: %v", err)
			}

			devs, err := device.New()
			if err != nil {
				t.Fatalf("New error: %v", err)
			}

			vfDevices := device.GetCDIDevices(devs)

			err = SyncDevices(cache, vfDevices)
			if err != nil {
				t.Fatalf("SyncDevices failed: %v", err)
			}
			testhelpers.CDICacheDelay()

			specs := cache.GetVendorSpecs(device.CDIVendor)
			var foundUIDs []string
			for _, spec := range specs {
				for _, d := range spec.Devices {
					foundUIDs = append(foundUIDs, d.Name)
				}
			}
			sort.Strings(foundUIDs)

			if len(foundUIDs) != len(tc.ExpectedUIDs) {
				t.Fatalf("Mismatch in number of devices: expected %d, got %d", len(tc.ExpectedUIDs), len(foundUIDs))
			}

			for i, foundUID := range foundUIDs {
				if foundUID != tc.ExpectedUIDs[i] {
					t.Errorf("Mismatch at index %d: expected %s, got %s", i, tc.ExpectedUIDs[i], foundUID)
				}
			}
		})
	}
}
