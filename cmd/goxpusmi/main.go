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

	"github.com/spf13/cobra"

	"github.com/intel/intel-resource-drivers-for-kubernetes/pkg/goxpusmi"
)

var (
	version = "v0.1.0"
)

func main() {
	command := newCommand()
	err := command.Execute()
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}

func cobraRunFunc(cmd *cobra.Command, args []string) error {
	if err := goxpusmi.Initialize(); err != nil {
		return fmt.Errorf("failed to initialize xpu-smi: %w", err)
	}

	// Do a verbose discovery.
	devices, err := goxpusmi.Discover(true)
	if err != nil {
		return fmt.Errorf("failed to print device number: %w", err)
	}

	fmt.Printf("Number of discovered devices: %d\n", len(devices))

	if err := goxpusmi.Shutdown(); err != nil {
		return fmt.Errorf("failed to shutdown xpu-smi: %w", err)
	}

	return nil
}

func newCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "goxpusmi",
		Short: "Go xpu-smi tester",
		Long:  "Test tool for xpu-smi Go bindings (goxpusmi)",
		RunE:  cobraRunFunc,
	}
	cmd.Version = version
	cmd.Flags().BoolP("version", "v", false, "Show the version of the binary")
	cmd.SetVersionTemplate("Test tool xpu-smi Go bindings (goxpusmi). Version: {{.Version}}\n")

	return cmd
}
