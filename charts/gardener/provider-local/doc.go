// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

//go:generate ../../../hack/generate-controller-registration.sh --pod-security-enforce=privileged provider-local . v0.0.0 ../../../example/provider-local/garden/base/controller-registration.yaml BackupBucket:local BackupEntry:local DNSRecord:local ControlPlane:local Infrastructure:local OperatingSystemConfig:local Worker:local

// Package chart enables go:generate support for generating the correct controller registration.
package chart
