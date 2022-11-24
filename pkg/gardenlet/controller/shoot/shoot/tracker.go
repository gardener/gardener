// Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package shoot

import (
	"sync"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ReconciliationDueTracker tracks due reconciliations.
type ReconciliationDueTracker struct {
	lock    sync.Mutex
	tracker map[client.ObjectKey]struct{}
}

func newReconciliationDueTracker() *ReconciliationDueTracker {
	return &ReconciliationDueTracker{tracker: make(map[client.ObjectKey]struct{})}
}

func (t *ReconciliationDueTracker) on(key client.ObjectKey) {
	t.lock.Lock()
	defer t.lock.Unlock()
	t.tracker[key] = struct{}{}
}

func (t *ReconciliationDueTracker) off(key client.ObjectKey) {
	t.lock.Lock()
	defer t.lock.Unlock()
	delete(t.tracker, key)
}

func (t *ReconciliationDueTracker) tracked(key client.ObjectKey) bool {
	t.lock.Lock()
	defer t.lock.Unlock()
	_, ok := t.tracker[key]
	return ok
}
