// Copyright 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

//go:generate ../../../hack/generate-controller-registration.sh --pod-security-enforce=privileged provider-local . v0.0.0 ../../../../example/provider-local/garden/base/controller-registration.yaml BackupBucket:local BackupEntry:local DNSRecord:local ControlPlane:local Infrastructure:local OperatingSystemConfig:local Worker:local

// Package chart enables go:generate support for generating the correct controller registration.
package chart
