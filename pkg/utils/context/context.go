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

package context

import (
	"context"
	"time"
)

// FromStopChannel creates a new context from a given stop channel.
func FromStopChannel(stopCh <-chan struct{}) context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		defer cancel()
		<-stopCh
	}()

	return ctx
}

type ops struct{}

// WithTimeout returns the context with the given timeout and a CancelFunc to cleanup its resources.
func (ops) WithTimeout(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, timeout)
}

// DefaultOps returns the default Ops implementation.
func DefaultOps() Ops {
	return ops{}
}
