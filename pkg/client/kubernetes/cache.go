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

package kubernetes

import (
	"sigs.k8s.io/controller-runtime/pkg/cache"
)

// NoOpCache holds an implementation of the controller-runtime cache and is used to turn specific functions
// into no-op functions by overriding them.
type NoOpCache struct {
	cache.Cache
}

// Start implements `Informers.Start` but only blocks until the given `stopCh` channel is closed
// w/o starting the cache. This is useful if the cache underneath has already been started.
func (n *NoOpCache) Start(stopCh <-chan struct{}) error {
	<-stopCh
	return nil
}
