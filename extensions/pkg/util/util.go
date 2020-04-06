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
// limitations under the License.

package util

import (
	"context"
	"encoding/json"

	"github.com/gardener/gardener/pkg/utils"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ContextFromStopChannel creates a new context from a given stop channel.
func ContextFromStopChannel(stopCh <-chan struct{}) context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		defer cancel()
		<-stopCh
	}()

	return ctx
}

// ComputeChecksum computes a SHA256 checksum for the give map.
func ComputeChecksum(data interface{}) string {
	jsonString, err := json.Marshal(data)
	if err != nil {
		return ""
	}
	return utils.ComputeSHA256Hex(jsonString)
}

// ObjectName returns the name of the given object in the format <namespace>/<name>
func ObjectName(obj runtime.Object) string {
	k, err := client.ObjectKeyFromObject(obj)
	if err != nil {
		return "/"
	}
	return k.String()
}

// Use can be used to mark go elements as used to temporaryly avoid compiler
// errors due to unused elements.
func Use(_ ...interface{}) {}
