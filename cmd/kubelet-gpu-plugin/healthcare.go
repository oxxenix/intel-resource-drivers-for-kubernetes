package main

import (
	"context"
	"time"

	"k8s.io/klog/v2"

	"github.com/intel/intel-resource-drivers-for-kubernetes/pkg/goxpusmi"
	"github.com/intel/intel-resource-drivers-for-kubernetes/pkg/gpu/device"
)

// updatedDeviceInfo contains the UID and description of an unhealthy device.
type updatedDeviceInfo struct {
	uid    string
	status map[string]string // key is health type, value is health status.
}

func (d *driver) startHealthMonitor(ctx context.Context, intervalSeconds int) {
	// Watch for device UIDs and descriptions to mark unhealthy.
	idsChan := make(chan updatedDeviceInfo)
	goxpusmiCtx, stopMonitor := context.WithCancel(ctx)
	go d.watchGPUHealthStatuses(goxpusmiCtx, intervalSeconds, idsChan)

	for {
		select {
		// Listen to original ctx, when driver is shutting down, stop HLML watcher.
		case <-goxpusmiCtx.Done():
			stopMonitor()
			return
		case updatedDevice := <-idsChan:
			d.updateHealth(goxpusmiCtx, updatedDevice)
		}
	}
}

func (d *driver) updateHealth(ctx context.Context, updatedDevice updatedDeviceInfo) {
	d.state.Lock()
	defer d.state.Unlock()

	klog.Infof("Updating info for device %v to status=%v", updatedDevice.uid, updatedDevice.status)
	//nolint:forcetypeassert // We want the code to panic if our assumption turns out to be wrong.
	allocatable := d.state.Allocatable.(map[string]*device.DeviceInfo)
	foundDevice, found := allocatable[updatedDevice.uid]
	if !found {
		klog.Errorf("could not find device with UID %v", updatedDevice.uid)
		return
	}

	// Determine overall health: healthy unless any status is CRITICAL.
	isHealthy := true
	if foundDevice.HealthStatus == nil {
		foundDevice.HealthStatus = make(map[string]string)
	}
	for healthType, status := range updatedDevice.status {
		foundDevice.HealthStatus[healthType] = status
		health := statusHealth(status)
		isHealthy = isHealthy && health
	}
	foundDevice.Healthy = isHealthy

	// Health is updated from a go routine, nothing we can do when publishing
	// resource slice fails, so error is only logged.
	if err := d.PublishResourceSlice(ctx); err != nil {
		klog.Errorf("could not publish updated resource slice: %v", err)
	}
}

// watchGPUHealthStatuses polls XPUM metric health info and updates health status of devices accordingly.
func (d *driver) watchGPUHealthStatuses(ctx context.Context, intervalSeconds int, idsChan chan<- updatedDeviceInfo) {
	devices, err := goxpusmi.Discover(true)
	if err != nil {
		klog.Errorf("could not discover devices for health monitoring: %v", err)
		return
	}

	healthCheckInterval := time.NewTicker(time.Duration(intervalSeconds) * time.Second)
	for {
		select {
		case <-ctx.Done():
			return
		case <-healthCheckInterval.C:
			if updates := goxpusmi.HealthCheck(devices); len(updates) > 0 {
				for uid, status := range updates {
					idsChan <- updatedDeviceInfo{uid: uid, status: status}
				}
			}
		}
	}
}

// statusHealth returns the health based on status value.
func statusHealth(status string) (health bool) {
	switch status {
	case "Critical":
		return false
	case "Warning":
		return true
	case "OK":
		return true
	case "Unknown":
		return true
	default:
		// something else than status value.
		panic("invalid status value")
	}
}
