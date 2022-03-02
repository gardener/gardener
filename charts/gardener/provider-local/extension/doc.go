// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

//go:generate ../../../../hack/generate-controller-registration.sh provider-local . v0.0.0 ../../../../example/provider-local/base/controller-registration.yaml BackupBucket:local BackupEntry:local DNSProvider:local DNSRecord:local ControlPlane:local Infrastructure:local OperatingSystemConfig:local Worker:local
//go:generate cp ../../../../example/provider-local/base/controller-registration.yaml ../../../../charts/gardener/provider-local/registration/templates/controller-registration.yaml
//go:generate sh -c "sed -i 's/    image:/{{ toYaml .Values.values | indent 4 }}/g' ../../../../charts/gardener/provider-local/registration/templates/controller-registration.yaml"
//go:generate sh -c "sed -i 's/      tag: .*//g' ../../../../charts/gardener/provider-local/registration/templates/controller-registration.yaml"

// Package chart enables go:generate support for generating the correct controller registration.
package chart
