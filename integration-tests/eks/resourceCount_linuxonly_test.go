// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

//go:build linuxonly
// +build linuxonly

package eks_addon

const (
	// Services count for CW agent on Linux and Windows
	serviceCountLinux   = 5
	serviceCountWindows = 3

	// DaemonSet count for CW agent on Linux and Windows
	daemonsetCountLinux   = 3
	daemonsetCountWindows = 2

	// Pods count for CW agent on Linux and Windows
	podCountLinux   = 3
	podCountWindows = 0
)
