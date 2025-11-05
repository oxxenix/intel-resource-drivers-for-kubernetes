package main

import (
	"context"
	"testing"
	"time"

	"github.com/intel/intel-resource-drivers-for-kubernetes/pkg/fakesysfs"
	gpudevice "github.com/intel/intel-resource-drivers-for-kubernetes/pkg/gpu/device"
	testhelpers "github.com/intel/intel-resource-drivers-for-kubernetes/pkg/plugintesthelpers"
)

// TestStartHealthMonitor is currently for coverage improvement only, as the internal
// watcher goroutine is hard to test in isolation. The test verifies that the
// startHealthMonitor goroutine can be started and stopped cleanly.
func TestStartHealthMonitor(t *testing.T) {
	testDirs, err := testhelpers.NewTestDirs(gpudevice.DriverName)
	defer testhelpers.CleanupTest(t, "GPU TestStartHealthMonitor", testDirs.TestRoot)
	if err != nil {
		t.Fatalf("setup error creating test dirs: %v", err)
	}

	testDevices := gpudevice.DevicesInfo{
		"0000-00-02-0-0x56c0": {Model: "0x56c0", MemoryMiB: 8192, DeviceType: "gpu", CardIdx: 0, RenderdIdx: 128, UID: "0000-00-02-0-0x56c0", MaxVFs: 16, Driver: "i915"},
	}
	if err := fakesysfs.FakeSysFsGpuContents(testDirs.SysfsRoot, testDirs.DevfsRoot, testDevices, false); err != nil {
		t.Fatalf("could not create fake sysfs: %v", err)
	}

	drv, err := getFakeDriver(testDirs)
	if err != nil {
		t.Fatalf("could not create fake driver: %v", err)
	}
	defer func() { _ = drv.Shutdown(context.TODO()) }()

	//nolint:forcetypeassert // We want the code to panic if our assumption turns out to be wrong.
	allocatable := drv.state.Allocatable.(map[string]*gpudevice.DeviceInfo)
	dev := allocatable["0000-00-02-0-0x56c0"]
	dev.Healthy = true

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		drv.startHealthMonitor(ctx, &GPUFlags{Healthcare: true, HealthcareInterval: 1}) // launches internal watcher and select loop
	}()

	// allow monitor to start
	time.Sleep(50 * time.Millisecond)

	// cancel and verify goroutine exit
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("startHealthMonitor goroutine did not exit")
	}
}

func TestUpdateHealth(t *testing.T) {
	testDirs, err := testhelpers.NewTestDirs(gpudevice.DriverName)
	defer testhelpers.CleanupTest(t, "GPU TestUpdateHealth", testDirs.TestRoot)
	if err != nil {
		t.Fatalf("setup error creating test dirs: %v", err)
	}

	testDevices := gpudevice.DevicesInfo{
		"0000-00-02-0-0x56c0": {Model: "0x56c0", MemoryMiB: 8192, DeviceType: "gpu", CardIdx: 0, RenderdIdx: 128, UID: "0000-00-02-0-0x56c0", MaxVFs: 16, Driver: "i915"},
	}
	if err := fakesysfs.FakeSysFsGpuContents(testDirs.SysfsRoot, testDirs.DevfsRoot, testDevices, false); err != nil {
		t.Fatalf("could not create fake sysfs: %v", err)
	}

	drv, err := getFakeDriver(testDirs)
	if err != nil {
		t.Fatalf("could not create fake driver: %v", err)
	}
	defer func() { _ = drv.Shutdown(context.TODO()) }()

	//nolint:forcetypeassert // We want the code to panic if our assumption turns out to be wrong.
	allocatable := drv.state.Allocatable.(map[string]*gpudevice.DeviceInfo)
	dev := allocatable["0000-00-02-0-0x56c0"]
	dev.Healthy = true

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tests := []struct {
		name                string
		ignoreHealthWarning bool
		updates             HealthStatusUpdates
		expectHealthy       bool
		expectStatuses      map[string]string
		expectPanic         bool
	}{
		{
			name: "Update to OK: healthy device remains healthy",
			updates: HealthStatusUpdates{
				"0000-00-02-0-0x56c0": {
					"CoreThermal":   "OK",
					"MemoryThermal": "OK",
					"Power":         "OK",
					"Memory":        "OK",
					"FabricPort":    "OK",
					"Frequency":     "OK",
				},
			},
			expectHealthy: true,
			expectStatuses: map[string]string{
				"CoreThermal":   "OK",
				"MemoryThermal": "OK",
				"Power":         "OK",
				"Memory":        "OK",
				"FabricPort":    "OK",
				"Frequency":     "OK",
			},
			ignoreHealthWarning: true,
		},
		{
			name: "Update to Unknown: healthy device becomes unhealthy",
			updates: HealthStatusUpdates{
				"0000-00-02-0-0x56c0": {
					"CoreThermal": "Unknown",
				},
			},
			expectHealthy: dev.Healthy,
			expectStatuses: map[string]string{
				"CoreThermal": "Unknown",
			},
			ignoreHealthWarning: true,
		},
		{
			name: "Update to Critical: healthy device becomes unhealthy",
			updates: HealthStatusUpdates{
				"0000-00-02-0-0x56c0": {
					"CoreThermal": "Critical",
				},
			},
			expectHealthy: false,
			expectStatuses: map[string]string{
				"CoreThermal": "Critical",
			},
			ignoreHealthWarning: true,
		},
		{
			name: "Update to Warning status with ignoreHealthWarning=true remains healthy",
			updates: HealthStatusUpdates{
				"0000-00-02-0-0x56c0": {
					"CoreThermal": "Warning",
				},
			},
			expectHealthy: dev.Healthy,
			expectStatuses: map[string]string{
				"CoreThermal": "Warning",
			},
			ignoreHealthWarning: true,
		},
		{
			name: "Update to Warning status with ignoreHealthWarning=false becomes unhealthy",
			updates: HealthStatusUpdates{
				"0000-00-02-0-0x56c0": {
					"CoreThermal": "Warning",
				},
			},
			expectHealthy: false,
			expectStatuses: map[string]string{
				"CoreThermal": "Warning",
			},
			ignoreHealthWarning: false,
		},
		{
			name: "Wrong device ID in update is ignored",
			updates: HealthStatusUpdates{
				"wrong-uid": {
					"CoreThermal": "Unexpected",
				},
			},
			expectHealthy: dev.Healthy,
			expectStatuses: map[string]string{
				"CoreThermal": "Warning",
			},
		}, {
			name: "Update to unexpected value",
			updates: HealthStatusUpdates{
				"0000-00-02-0-0x56c0": {
					"CoreThermal": "Unexpected",
				},
			},
			expectPanic:         true,
			ignoreHealthWarning: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Handle expected panic
			if tt.expectPanic {
				defer func() {
					if r := recover(); r == nil {
						t.Fatal("expected panic for invalid status, but no panic occurred")
					}
				}()
			}

			// Reset device to healthy state before each test
			dev.Healthy = true

			// Set the driver's ignoreHealthWarning setting
			drv.state.ignoreHealthWarning = tt.ignoreHealthWarning

			drv.updateHealth(ctx, tt.updates)

			// Skip assertions if we're expecting a panic
			if tt.expectPanic {
				return
			}

			if dev.Healthy != tt.expectHealthy {
				t.Fatalf("expected device healthy=%v, got %v", tt.expectHealthy, dev.Healthy)
			}

			for status, expected := range tt.expectStatuses {
				if dev.HealthStatus[status] != expected {
					t.Fatalf("expected health status for %s to be %s, got %s", status, expected, dev.HealthStatus[status])
				}
			}
		})
	}
}

func TestUpdateHealth_MultipleDevices(t *testing.T) {
	testDirs, err := testhelpers.NewTestDirs(gpudevice.DriverName)
	defer testhelpers.CleanupTest(t, "GPU TestUpdateHealth_MultipleDevices", testDirs.TestRoot)
	if err != nil {
		t.Fatalf("setup error creating test dirs: %v", err)
	}

	testDevices := gpudevice.DevicesInfo{
		"0000-00-02-0-0x56c0": {Model: "0x56c0", MemoryMiB: 8192, DeviceType: "gpu", CardIdx: 0, RenderdIdx: 128, UID: "0000-00-02-0-0x56c0", MaxVFs: 16, Driver: "i915"},
		"0000-00-03-0-0x56c1": {Model: "0x56c1", MemoryMiB: 8192, DeviceType: "gpu", CardIdx: 1, RenderdIdx: 129, UID: "0000-00-03-0-0x56c1", MaxVFs: 16, Driver: "i915"},
	}
	if err := fakesysfs.FakeSysFsGpuContents(testDirs.SysfsRoot, testDirs.DevfsRoot, testDevices, false); err != nil {
		t.Fatalf("could not create fake sysfs: %v", err)
	}

	drv, err := getFakeDriver(testDirs)
	if err != nil {
		t.Fatalf("could not create fake driver: %v", err)
	}
	defer func() { _ = drv.Shutdown(context.TODO()) }()

	//nolint:forcetypeassert // We want the code to panic if our assumption turns out to be wrong.
	allocatable := drv.state.Allocatable.(map[string]*gpudevice.DeviceInfo)
	dev1 := allocatable["0000-00-02-0-0x56c0"]
	dev2 := allocatable["0000-00-03-0-0x56c1"]

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tests := []struct {
		name                string
		initialHealthy      bool
		ignoreHealthWarning bool
		updates             HealthStatusUpdates
		expectHealthyDev1   bool
		expectStatusesDev1  map[string]string
		expectHealthyDev2   bool
		expectStatusesDev2  map[string]string
	}{
		{
			name:                "Isolated failure: one unhealthy device disables only that device",
			initialHealthy:      true,
			ignoreHealthWarning: false,
			updates: HealthStatusUpdates{
				"0000-00-03-0-0x56c1": {
					"CoreThermal": "Critical",
				},
			},
			expectHealthyDev1:  true,
			expectStatusesDev1: map[string]string{},
			expectHealthyDev2:  false,
			expectStatusesDev2: map[string]string{"CoreThermal": "Critical"},
		},
		{
			name:                "Partial recovery: only one device becomes healthy while the other stays unhealthy",
			initialHealthy:      false,
			ignoreHealthWarning: false,
			updates: HealthStatusUpdates{
				"0000-00-03-0-0x56c1": {
					"CoreThermal": "OK",
				},
			},
			expectHealthyDev1:  false,
			expectHealthyDev2:  true,
			expectStatusesDev2: map[string]string{"CoreThermal": "OK"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			// Reset device to healthy state before each test
			dev1.Healthy = tt.initialHealthy
			dev2.Healthy = tt.initialHealthy

			// Set the driver's ignoreHealthWarning setting
			drv.state.ignoreHealthWarning = tt.ignoreHealthWarning

			drv.updateHealth(ctx, tt.updates)

			if dev1.Healthy != tt.expectHealthyDev1 {
				t.Fatalf("expected device1 healthy=%v, got %v", tt.expectHealthyDev1, dev1.Healthy)
			}

			for status, expected := range tt.expectStatusesDev1 {
				if dev1.HealthStatus[status] != expected {
					t.Fatalf("expected health status for %s to be %s, got %s", status, expected, dev1.HealthStatus[status])
				}
			}

			if dev2.Healthy != tt.expectHealthyDev2 {
				t.Fatalf("expected device2 healthy=%v, got %v", tt.expectHealthyDev2, dev2.Healthy)
			}

			for status, expected := range tt.expectStatusesDev2 {
				if dev2.HealthStatus[status] != expected {
					t.Fatalf("expected health status for %s to be %s, got %s", status, expected, dev2.HealthStatus[status])
				}
			}
		})
	}
}
