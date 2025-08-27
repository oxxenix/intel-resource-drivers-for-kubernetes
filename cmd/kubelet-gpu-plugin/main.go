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

package main

import (
	"fmt"
	"os"

	"github.com/urfave/cli/v2"

	"github.com/intel/intel-resource-drivers-for-kubernetes/pkg/gpu/device"
	"github.com/intel/intel-resource-drivers-for-kubernetes/pkg/helpers"
)

const (
	HealthCareFlagDefault         = false
	HealthcareIntervalFlagMin     = 1
	HealthcareIntervalFlagMax     = 3600
	HealthcareIntervalFlagDefault = 5
)

type GPUFlags struct {
	Partitioning       bool
	Healthcare         bool
	HealthcareInterval int
}

const (
	PartitioningDefault = false
)

func main() {
	gpuFlags := GPUFlags{
		Partitioning:       PartitioningDefault,
		Healthcare:         HealthCareFlagDefault,
		HealthcareInterval: HealthcareIntervalFlagDefault,
	}
	cliFlags := []cli.Flag{
		&cli.BoolFlag{
			Name:        "health-monitoring",
			Aliases:     []string{"m"},
			Usage:       "Actively monitor device health and update ResourceSlice. Requires privileges.",
			Value:       HealthCareFlagDefault,
			Destination: &gpuFlags.Healthcare,
			EnvVars:     []string{"HEALTH_MONITORING"},
		},
		&cli.IntFlag{
			Name:        "health-interval",
			Aliases:     []string{"i"},
			Usage:       fmt.Sprintf("Number of seconds between health-monitoring checks [%v ~ %v]", HealthcareIntervalFlagMin, HealthcareIntervalFlagMax),
			Value:       HealthcareIntervalFlagDefault,
			Destination: &gpuFlags.HealthcareInterval,
			EnvVars:     []string{"HEALTH_INTERVAL"},
		},
		&cli.BoolFlag{
			Name:        "partitioning-management",
			Aliases:     []string{"p"},
			Usage:       "Manage partitioning physical devices into virtual. [Not Supported]",
			Value:       PartitioningDefault,
			Destination: &gpuFlags.Partitioning,
			EnvVars:     []string{"PARTITIONING"},
		},
	}

	if err := helpers.NewApp(device.DriverName, newDriver, cliFlags, &gpuFlags).Run(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
