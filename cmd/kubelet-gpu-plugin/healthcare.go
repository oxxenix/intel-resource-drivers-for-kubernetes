package main

import (
	"context"
	"fmt"
	"time"

	resourceapi "k8s.io/api/resource/v1alpha3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"

	"github.com/intel/intel-resource-drivers-for-kubernetes/pkg/goxpusmi"
	"github.com/intel/intel-resource-drivers-for-kubernetes/pkg/gpu/device"
)

// updatedDeviceInfo contains the UID and description of an unhealthy device.
type updatedDeviceInfo struct {
	uid    string
	status map[int]int // key is health type, value is health status.
}

// Health type identifiers. Order must match xpum structs.
const (
	CORE_THERMAL int = iota
	MEMORY_THERMAL
	POWER
	MEMORY
	FABRIC_PORT
	FREQUENCY
)

// Health status values. Order must match xpum structs.
const (
	UNKNOWN int = iota
	OK
	WARNING
	CRITICAL
)

// buildTaintRuleName returns the unique DeviceTaintRule name for a specific device health type.
// Format kept stable for backward compatibility with previously created rules.
func buildTaintRuleName(driverName, nodeName, deviceUID string, healthType int) string {
	return fmt.Sprintf("%s-%s-%s-%d", driverName, nodeName, deviceUID, healthType)
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

	allocatable, _ := d.state.Allocatable.(map[string]*device.DeviceInfo)
	foundDevice, found := allocatable[updatedDevice.uid]
	if !found {
		klog.Errorf("could not find device with UID %v", updatedDevice.uid)
		return
	}

	// Determine overall health: healthy unless any status is CRITICAL.
	isHealthy := true
	for healthType, status := range updatedDevice.status {
		d.updateTaintRule(ctx, updatedDevice.uid, healthType, status)
		_, _, health, _ := statusNameTaintHealthEffect(status)
		isHealthy = isHealthy && health
	}
	foundDevice.Healthy = isHealthy

	// Health is updated from a go routine, nothing we can do when publishing
	// resource slice fails, so error is ignored.
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
			if updated, updates := goxpusmi.HealthCheck(devices); updated {
				for uid, status := range updates {
					idsChan <- updatedDeviceInfo{uid: uid, status: status}
				}
			}
		}
	}
}

// updateTaintRule updates a DeviceTaintRule for a device whose health status has changed.
func (d *driver) updateTaintRule(ctx context.Context, updatedDeviceUID string, healthType, status int) {
	taintValue, taint, _, taintEffect := statusNameTaintHealthEffect(status)
	if !taint {
		d.deleteTaintRule(ctx, updatedDeviceUID, healthType)
		return
	}

	driverName := device.DriverName
	nodeName := d.state.NodeName
	taintRuleName := buildTaintRuleName(driverName, nodeName, updatedDeviceUID, healthType)
	if d.doesTaintRuleExist(ctx, taintRuleName) {
		klog.FromContext(ctx).Info("DeviceTaintRule already exists, skip creating", "ruleName", taintRuleName)
		return
	}

	// Build taint attributes.
	taintKey := taintKeyForHealthType(driverName, healthType)

	devTaintRule := resourceapi.DeviceTaintRule{
		ObjectMeta: metav1.ObjectMeta{
			Name: taintRuleName,
		},
		Spec: resourceapi.DeviceTaintRuleSpec{
			DeviceSelector: &resourceapi.DeviceTaintSelector{
				Driver: &driverName,
				Pool:   &nodeName,
				Device: &updatedDeviceUID,
			},
			Taint: resourceapi.DeviceTaint{
				Key:    taintKey,
				Value:  taintValue,
				Effect: taintEffect,
			},
		},
	}

	klog.FromContext(ctx).Info("creating DeviceTaintRule", "rule", devTaintRule)
	_, err := d.client.ResourceV1alpha3().DeviceTaintRules().Create(ctx, &devTaintRule, metav1.CreateOptions{})
	if err != nil {
		klog.Errorf("failed to create '%v' device taint rule: %v", taintKey, err)
	}
}

// deleteTaintRule removes specified DeviceTaintRule for a device.
func (d *driver) deleteTaintRule(ctx context.Context, updatedDeviceUID string, healthType int) {
	taintRuleName := buildTaintRuleName(device.DriverName, d.state.NodeName, updatedDeviceUID, healthType)
	if !d.doesTaintRuleExist(ctx, taintRuleName) {
		klog.FromContext(ctx).Info("DeviceTaintRule does not exist, nothing to delete", "ruleName", taintRuleName)
		return
	}

	err := d.client.ResourceV1alpha3().DeviceTaintRules().Delete(ctx, taintRuleName, metav1.DeleteOptions{})
	if err != nil {
		klog.Errorf("failed to delete device taint rule %s: %v", taintRuleName, err)
	} else {
		klog.FromContext(ctx).Info("deleted DeviceTaintRule", "ruleName", taintRuleName)
	}
}

// Check if the rule already exists, or new rule creation will fail because of the name conflict.
func (d *driver) doesTaintRuleExist(ctx context.Context, taintRuleName string) bool {
	rule, err := d.client.ResourceV1alpha3().DeviceTaintRules().Get(ctx, taintRuleName, metav1.GetOptions{})
	if err == nil && rule != nil {
		klog.FromContext(ctx).Info("Found existing DeviceTaintRule", "rule", rule)
		return true
	}
	if err != nil {
		klog.Infof("failed to get device taint rule %s: %v", taintRuleName, err)
	}
	return false
}

// taintKeyForHealthType builds a taint key for the given health type.
func taintKeyForHealthType(driver string, healthType int) string {
	suffix := "unknown"
	switch healthType {
	case CORE_THERMAL:
		suffix = "core-thermal"
	case MEMORY_THERMAL:
		suffix = "memory-thermal"
	case POWER:
		suffix = "power"
	case MEMORY:
		suffix = "memory"
	case FABRIC_PORT:
		suffix = "fabric-port"
	case FREQUENCY:
		suffix = "frequency"
	}
	return fmt.Sprintf("%s/%s", driver, suffix)
}

// statusNameTaintHealthEffect returns the name, taint, health and effect info based on status value
func statusNameTaintHealthEffect(status int) (name string, taint bool, health bool, effect resourceapi.DeviceTaintEffect) {
	switch status {
	case CRITICAL:
		return "Critical", true, false, resourceapi.DeviceTaintEffectNoExecute
	case WARNING:
		return "Warning", true, true, resourceapi.DeviceTaintEffectNoSchedule
	case OK:
		return "OK", false, true, ""
	case UNKNOWN:
		return "Unknown", false, true, ""
	default:
		// something else than status value.
		panic("invalid status value")
	}
}
