// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package healthz

import (
	"context"
	"errors"
	"net/http"

	"sigs.k8s.io/controller-runtime/pkg/healthz"
)

// Manager is an interface for health managers.
type Manager interface {
	// Name returns the name of the health manager.
	Name() string
	// Start starts the health manager.
	Start(context.Context) error
	// Stop stops the health manager.
	Stop()
	// Get returns the current health status.
	Get() bool
	// Set updates the current health status with the given value.
	Set(bool)
}

// CheckerFunc returns a new healthz.Checker that will pass only if the given health manager returns true.
func CheckerFunc(h Manager) healthz.Checker {
	return func(_ *http.Request) error {
		if !h.Get() {
			return errors.New("current health status is 'unhealthy'")
		}
		return nil
	}
}
