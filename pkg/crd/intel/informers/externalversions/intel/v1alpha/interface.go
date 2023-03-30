/*
 * Copyright (c) 2023, Intel Corporation.  All Rights Reserved.
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

// Code generated by informer-gen. DO NOT EDIT.

package v1alpha

import (
	internalinterfaces "github.com/intel/intel-resource-drivers-for-kubernetes/pkg/crd/intel/informers/externalversions/internalinterfaces"
)

// Interface provides access to all the informers in this group version.
type Interface interface {
	// DeviceClassParameters returns a DeviceClassParametersInformer.
	DeviceClassParameters() DeviceClassParametersInformer
	// GpuAllocationStates returns a GpuAllocationStateInformer.
	GpuAllocationStates() GpuAllocationStateInformer
	// GpuClaimParameters returns a GpuClaimParametersInformer.
	GpuClaimParameters() GpuClaimParametersInformer
}

type version struct {
	factory          internalinterfaces.SharedInformerFactory
	namespace        string
	tweakListOptions internalinterfaces.TweakListOptionsFunc
}

// New returns a new Interface.
func New(f internalinterfaces.SharedInformerFactory, namespace string, tweakListOptions internalinterfaces.TweakListOptionsFunc) Interface {
	return &version{factory: f, namespace: namespace, tweakListOptions: tweakListOptions}
}

// DeviceClassParameters returns a DeviceClassParametersInformer.
func (v *version) DeviceClassParameters() DeviceClassParametersInformer {
	return &deviceClassParametersInformer{factory: v.factory, tweakListOptions: v.tweakListOptions}
}

// GpuAllocationStates returns a GpuAllocationStateInformer.
func (v *version) GpuAllocationStates() GpuAllocationStateInformer {
	return &gpuAllocationStateInformer{factory: v.factory, namespace: v.namespace, tweakListOptions: v.tweakListOptions}
}

// GpuClaimParameters returns a GpuClaimParametersInformer.
func (v *version) GpuClaimParameters() GpuClaimParametersInformer {
	return &gpuClaimParametersInformer{factory: v.factory, namespace: v.namespace, tweakListOptions: v.tweakListOptions}
}
