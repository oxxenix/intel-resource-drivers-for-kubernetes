/* Copyright (C) 2024 Intel Corporation
 * SPDX-License-Identifier: Apache-2.0
 */

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"testing"

	core "k8s.io/api/core/v1"
	resourcev1 "k8s.io/api/resource/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	kubefake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/dynamic-resource-allocation/kubeletplugin"

	"github.com/intel/intel-resource-drivers-for-kubernetes/pkg/fakesysfs"
	helpers "github.com/intel/intel-resource-drivers-for-kubernetes/pkg/helpers"
	testhelpers "github.com/intel/intel-resource-drivers-for-kubernetes/pkg/plugintesthelpers"
	"github.com/intel/intel-resource-drivers-for-kubernetes/pkg/qat/device"
)

const (
	testNodeName  = "test-node-01"
	testNameSpace = "test-namespace-01"
)

func getFakeDriver(testDirs testhelpers.TestDirsType) (*driver, error) {
	config := &helpers.Config{
		CommonFlags: &helpers.Flags{
			NodeName:                  testNodeName,
			CdiRoot:                   testDirs.CdiRoot,
			KubeletPluginDir:          testDirs.KubeletPluginDir,
			KubeletPluginsRegistryDir: testDirs.KubeletPluginRegistryDir,
		},
		Coreclient:  kubefake.NewSimpleClientset(),
		DriverFlags: nil,
	}

	if err := os.MkdirAll(config.CommonFlags.KubeletPluginDir, 0755); err != nil {
		return nil, fmt.Errorf("failed creating fake driver plugin dir: %v", err)
	}
	if err := os.MkdirAll(config.CommonFlags.KubeletPluginsRegistryDir, 0755); err != nil {
		return nil, fmt.Errorf("failed creating fake driver plugin dir: %v", err)
	}

	os.Setenv("SYSFS_ROOT", testDirs.SysfsRoot)

	// kubelet-plugin will access node object, it needs to exist.
	newNode := &core.Node{ObjectMeta: metav1.ObjectMeta{Name: testNodeName}}
	if _, err := config.Coreclient.CoreV1().Nodes().Create(context.TODO(), newNode, metav1.CreateOptions{}); err != nil {
		return nil, fmt.Errorf("failed creating fake node object: %v", err)
	}

	helperdriver, err := newDriver(context.TODO(), config)
	if err != nil {
		return nil, fmt.Errorf("failed creating driver object: %v", err)
	}

	driver, ok := helperdriver.(*driver)
	if !ok {
		return nil, fmt.Errorf("type assertion failed: expected driver, got %T", driver)
	}
	return driver, err
}

func TestDriver(t *testing.T) {
	type testCase struct {
		name             string
		request          []*resourcev1.ResourceClaim
		expectedResponse map[types.UID]kubeletplugin.PrepareResult
	}

	setupdevices := fakesysfs.QATDevices{
		{Device: "0000:aa:00.0",
			State:    "up",
			Services: "sym;asym",
			TotalVFs: 3,
			NumVFs:   0,
		},
		{Device: "0000:bb:00.0",
			State:    "up",
			Services: "dc",
			TotalVFs: 3,
			NumVFs:   0,
		},
	}

	defer fakesysfs.FakeFsRemove()
	if err := fakesysfs.FakeSysFsQATContents(setupdevices); err != nil {
		t.Fatalf("err: %v", err)
	}

	testcases := []testCase{
		{
			name: "QAT allocate device",
			request: []*resourcev1.ResourceClaim{
				testhelpers.NewClaim(testNameSpace, "claim1", "uid1", "request1", "qat.intel.com", testNodeName, []string{"qatvf-0000-aa-00-1"}),
			},
			expectedResponse: map[types.UID]kubeletplugin.PrepareResult{
				"uid1": {
					Devices: []kubeletplugin.Device{
						{Requests: []string{"request1"}, PoolName: testNodeName, DeviceName: "qatvf-0000-aa-00-1", CDIDeviceIDs: []string{"intel.com/qat=qatvf-0000-aa-00-1", "intel.com/qat=qatvf-vfio"}},
					},
				},
			},
		},
		{
			name: "QAT reallocate same device and same claim UID",
			request: []*resourcev1.ResourceClaim{
				testhelpers.NewClaim(testNameSpace, "claim-a", "uid1", "request1", "qat.intel.com", testNodeName, []string{"qatvf-0000-aa-00-1"}),
			},
			expectedResponse: map[types.UID]kubeletplugin.PrepareResult{
				"uid1": {
					Devices: []kubeletplugin.Device{
						{Requests: []string{"request1"}, PoolName: testNodeName, DeviceName: "qatvf-0000-aa-00-1", CDIDeviceIDs: []string{"intel.com/qat=qatvf-0000-aa-00-1", "intel.com/qat=qatvf-vfio"}},
					},
				},
			},
		},
		{
			name: "QAT device already allocated",
			request: []*resourcev1.ResourceClaim{
				testhelpers.NewClaim(testNameSpace, "claim2", "uid2", "request1", "qat.intel.com", testNodeName, []string{"qatvf-0000-aa-00-1"}),
			},
			expectedResponse: map[types.UID]kubeletplugin.PrepareResult{
				"uid2": {
					Err: fmt.Errorf("could not allocate device 'qatvf-0000-aa-00-1', service '' from any device"),
				},
			},
		},
		{
			name: "QAT two devices",
			request: []*resourcev1.ResourceClaim{
				testhelpers.NewClaim(testNameSpace, "claim3", "uid1", "request3", "qat.intel.com", testNodeName, []string{"qatvf-0000-aa-00-3", "qatvf-0000-bb-00-1"}),
			},
			expectedResponse: map[types.UID]kubeletplugin.PrepareResult{
				"uid1": {
					Devices: []kubeletplugin.Device{
						{Requests: []string{"request3"}, PoolName: testNodeName, DeviceName: "qatvf-0000-aa-00-3", CDIDeviceIDs: []string{"intel.com/qat=qatvf-0000-aa-00-3", "intel.com/qat=qatvf-vfio"}},
						{Requests: []string{"request3"}, PoolName: testNodeName, DeviceName: "qatvf-0000-bb-00-1", CDIDeviceIDs: []string{"intel.com/qat=qatvf-0000-bb-00-1", "intel.com/qat=qatvf-vfio"}},
					},
				},
			},
		},
	}

	for _, testcase := range testcases {
		testDirs, err := testhelpers.NewTestDirs(device.DriverName)
		defer testhelpers.CleanupTest(t, testcase.name, testDirs.TestRoot)

		driver, err := getFakeDriver(testDirs)
		if err != nil {
			t.Fatalf("could not create qatdevices with New(): %v", err)
		}

		t.Log(testcase.name)

		response, err := driver.PrepareResourceClaims(context.TODO(), testcase.request)
		if err != nil {
			t.Errorf("%v: error %v, expected no error", testcase.name, err)
			continue
		}

		if !reflect.DeepEqual(testcase.expectedResponse, response) {
			responseJSON, _ := json.MarshalIndent(response, "", "\t")
			expectedResponseJSON, _ := json.MarshalIndent(testcase.expectedResponse, "", "\t")
			t.Errorf("%v: unexpected response: %+v, expected response: %v", testcase.name, string(responseJSON), string(expectedResponseJSON))
		}
	}
}
