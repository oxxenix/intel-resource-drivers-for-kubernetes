/*
 * Copyright (c) 2024, Intel Corporation.  All Rights Reserved.
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
	"fmt"
	"path"

	"k8s.io/klog/v2"
	cdiapi "tags.cncf.io/container-device-interface/pkg/cdi"
	cdispecs "tags.cncf.io/container-device-interface/specs-go"

	"github.com/intel/intel-resource-drivers-for-kubernetes/pkg/qat/device"
)

func getQatSpecs(cdiCache *cdiapi.Cache) []*cdiapi.Spec {
	qatSpecs := []*cdiapi.Spec{}
	for _, cdiSpec := range cdiCache.GetVendorSpecs(device.CDIVendor) {
		if cdiSpec.Kind == device.CDIKind {
			qatSpecs = append(qatSpecs, cdiSpec)
		}
	}
	return qatSpecs
}

func SyncDevices(cdiCache *cdiapi.Cache, vfdevices device.VFDevices) error {
	klog.V(5).Info("Syncing CDI devices")

	vfspec := &cdispecs.Spec{
		Kind: device.CDIKind,
	}
	vfspecname := cdiapi.GenerateSpecName(device.CDIVendor, device.CDIClass)

	for _, vendorspec := range getQatSpecs(cdiCache) {
		vendorspecname := path.Base(vendorspec.GetPath())

		name := vfspecname + path.Ext(vendorspecname)
		if name == vendorspecname {
			klog.V(5).Infof("Adding rest of the devices to '%s'", name)
			vfspec = vendorspec.Spec
		}

		vendorspecupdate := false
		vendorspecdevices := []cdispecs.Device{}

		for _, vendordevice := range vendorspec.Devices {
			if _, exists := vfdevices[vendordevice.Name]; exists {
				klog.V(5).Infof("Vendor spec %s contains device name %s", vendorspecname, vendordevice.Name)

				delete(vfdevices, vendordevice.Name)
				vendorspecdevices = append(vendorspecdevices, vendordevice)
			} else {
				klog.Warningf("CDI device '%s' in spec file '%s' does not exist", vendordevice.Name, vendorspecname)
				vendorspecupdate = true
			}
		}
		if vendorspecupdate {
			vendorspec.Devices = vendorspecdevices

			if len(vendorspec.Devices) == 0 {
				klog.V(5).Infof("No devices in spec %v, deleting it", vendorspecname)
				if err := cdiCache.RemoveSpec(vendorspecname); err != nil {
					klog.Errorf("failed to remove empty CDI spec %v: %v", vendorspecname, err)
				}
				continue
			}

			// Update spec file that has a nonexistent device.
			klog.Infof("Updating spec file %s with existing devices", path.Base(vendorspec.GetPath()))
			if err := cdiCache.WriteSpec(vendorspec.Spec, vendorspecname); err != nil {
				klog.Errorf("Failed to update existing CDI spec file %s: %v", vendorspecname, err)
			}
		}
	}

	if len(vfdevices) > 0 {
		return appendDevices(cdiCache, vfspec, vfdevices, vfspecname)
	}

	return nil
}

func addDeviceSpec(spec *cdispecs.Spec, vfdevices device.VFDevices) error {

	for _, vf := range vfdevices {
		cdidevice := cdispecs.Device{
			Name: vf.UID(),
			ContainerEdits: cdispecs.ContainerEdits{
				DeviceNodes: []*cdispecs.DeviceNode{
					{Path: vf.DeviceNode(), Type: "c"},
				},
			},
		}
		spec.Devices = append(spec.Devices, cdidevice)

		klog.V(5).Infof("Added device %s name %s", cdidevice.ContainerEdits.DeviceNodes[0].Path, cdidevice.Name)
	}
	return nil
}

func appendDevices(cdiCache *cdiapi.Cache, spec *cdispecs.Spec, vfdevices device.VFDevices, name string) error {

	klog.V(5).Info("Append CDI devices")

	if err := addDeviceSpec(spec, vfdevices); err != nil {
		return err
	}

	version, err := cdiapi.MinimumRequiredVersion(spec)
	if err != nil {
		return fmt.Errorf("minimum CDI spec version not found: %v", err)
	}
	spec.Version = version

	err = cdiCache.WriteSpec(spec, name)
	if err != nil {
		return fmt.Errorf("failed to write CDI spec %s: %v", name, err)
	}

	klog.Infof("CDI %s: Kind %s, Version %v", name, spec.Kind, spec.Version)
	return nil
}
