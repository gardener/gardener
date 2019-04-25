// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

//go:generate mockgen -package=context -destination=funcs.go github.com/gardener/gardener/pkg/mock/go/context WithTimeout,CancelFunc

package context

import (
	"context"
	"time"
)

// WithTimeout is an interface that allows mocking `WithTimeout`.
type WithTimeout interface {
	Do(parent context.Context, timeout time.Duration) (context.Context, context.CancelFunc)
}

// CancelFunc is an interface that allows mocking `CancelFunc`.
type CancelFunc interface {
	Do()
}
