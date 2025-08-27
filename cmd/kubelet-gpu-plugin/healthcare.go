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

func (d *driver) startHealthMonitor(ctx context.Context, intervalSeconds int) {
	// Watch for device UIDs to mark unhealthy.
	idsChan := make(chan string)
	goxpusmiCtx, stopMonitor := context.WithCancel(ctx)
	go d.watchGPUHealthStatuses(goxpusmiCtx, intervalSeconds, idsChan)

	for {
		select {
		// Listen to original ctx, when driver is shutting down, stop HLML watcher.
		case <-goxpusmiCtx.Done():
			stopMonitor()
			return
		case unhealthyUID := <-idsChan:
			d.updateHealth(goxpusmiCtx, false, unhealthyUID)
		}
	}
}

func (d *driver) updateHealth(ctx context.Context, healthy bool, uid string) {
	d.state.Lock()
	defer d.state.Unlock()

	allocatable, _ := d.state.Allocatable.(map[string]*device.DeviceInfo)
	foundDevice, found := allocatable[uid]
	if !found {
		klog.Errorf("could not find device with UID %v", uid)
		return
	}

	d.createTaintRuleMaybe(ctx, uid)

	foundDevice.Healthy = healthy
	// Health is updated from a go routine, nothing we can do when publishing
	// resource slice fails, so error is ignored.
	if err := d.PublishResourceSlice(ctx); err != nil {
		klog.Errorf("could not publish updated resource slice: %v", err)
	}
}

// watchGPUHealthStatuses watches for health events and marks the devices as unhealthy.
func (d *driver) watchGPUHealthStatuses(ctx context.Context, intervalSeconds int, idsChan chan<- string) {
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
			if pushUIDs, uids := goxpusmi.HealthCheck(devices); pushUIDs {
				for _, uid := range uids {
					idsChan <- uid
				}
			}
		}
	}
}

// createTaintRuleMaybe ensures there is a DeviceTaintRule for the device that
// became unhealthy.
func (d *driver) createTaintRuleMaybe(ctx context.Context, uid string) {
	taintRuleName := fmt.Sprintf("%v-%v-%v", device.DriverName, d.state.NodeName, uid)
	driverName := device.DriverName
	// Taint failed device, so it will not be scheduled.
	devTaintRule := resourceapi.DeviceTaintRule{
		ObjectMeta: metav1.ObjectMeta{
			Name: taintRuleName,
		},
		Spec: resourceapi.DeviceTaintRuleSpec{
			DeviceSelector: &resourceapi.DeviceTaintSelector{
				Driver: &driverName,
				Pool:   &d.state.NodeName,
				Device: &uid,
			},
			Taint: resourceapi.DeviceTaint{
				Key:    fmt.Sprintf("%s/unhealthy", device.DriverName),
				Value:  "CriticalError",
				Effect: resourceapi.DeviceTaintEffectNoExecute,
			},
		},
	}

	// Check if the rule already exists, or new rule creation will fail because of the name conflict.
	rule, err := d.client.ResourceV1alpha3().DeviceTaintRules().Get(ctx, taintRuleName, metav1.GetOptions{})
	if err == nil && rule != nil {
		klog.FromContext(ctx).Info("Found existing DeviceTaintRule", "rule", rule)
		return
	}

	klog.FromContext(ctx).Info("creating DeviceTaintRule", "rule", devTaintRule)
	_, err = d.client.ResourceV1alpha3().DeviceTaintRules().Create(ctx, &devTaintRule, metav1.CreateOptions{})
	if err != nil {
		klog.Errorf("failed to create device taint rule: %v", err)
	}
}
