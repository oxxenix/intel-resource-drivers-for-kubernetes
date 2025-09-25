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
	if err != nil {
		t.Fatalf("setup error creating test dirs: %v", err)
	}
	defer testhelpers.CleanupTest(t, "TestWatchGPUHealthStatuses", testDirs.TestRoot)

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
	if err != nil {
		t.Fatalf("setup error creating test dirs: %v", err)
	}
	defer testhelpers.CleanupTest(t, "TestGpuUpdateHealth", testDirs.TestRoot)

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
