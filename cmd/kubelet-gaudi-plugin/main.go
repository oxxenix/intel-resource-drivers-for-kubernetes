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

	gaudi "github.com/intel/intel-resource-drivers-for-kubernetes/pkg/gaudi/device"
	"github.com/intel/intel-resource-drivers-for-kubernetes/pkg/helpers"
)

type GaudiFlags struct {
	GaudiHookPath string
	GaudinetPath  string
}

func main() {
	gaudiFlags := GaudiFlags{
		GaudiHookPath: gaudi.DefaultHabanaHookPath,
		GaudinetPath:  gaudi.DefaultGaudinetPath,
	}
	cliFlags := []cli.Flag{
		&cli.StringFlag{
			Name:        "gaudi-hook-path",
			Aliases:     []string{"p"},
			Usage:       "full path to the habana-container-hook",
			Value:       gaudi.DefaultHabanaHookPath,
			Destination: &gaudiFlags.GaudiHookPath,
			EnvVars:     []string{"GAUDI_HOOK_PATH"},
		},
		&cli.StringFlag{
			Name:        "gaudinet-path",
			Aliases:     []string{"n"},
			Usage:       "full path to the network configuration file",
			Value:       gaudi.DefaultGaudinetPath,
			Destination: &gaudiFlags.GaudinetPath,
			EnvVars:     []string{"GAUDINET_PATH"},
		},
	}

	if err := helpers.NewApp(gaudi.DriverName, newDriver, cliFlags, &gaudiFlags).Run(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
