// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package framework

import "k8s.io/api/core/v1"

func (opts *TillerOptions) pullPolicy() v1.PullPolicy {
	if opts.UseCanary {
		return v1.PullAlways
	}
	return v1.PullIfNotPresent
}

func (opts *TillerOptions) getReplicas() int32 {
	replicas := int32(1)
	if opts.Replicas > 1 {
		replicas = int32(opts.Replicas)
	}
	return replicas
}
