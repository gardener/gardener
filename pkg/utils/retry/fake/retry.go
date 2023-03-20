// Copyright 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package fake

import (
	"context"
	"fmt"
	"time"

	"github.com/gardener/gardener/pkg/utils/retry"
)

var _ retry.Ops = &Ops{}

// Ops implements retry.Ops and can be used to mock calls to retry.Until and retry.UntilTimeout in unit tests.
// This implementation ignores the `interval` parameter and doesn't wait between retries, which makes it useful for
// writing quick and stable unit tests.
type Ops struct {
	// MaxAttempts configures the maximum amount of attempts before returning a retryError. If it is set to 0, it
	// fails immediately and f is never called.
	MaxAttempts int
}

// Until implements retry.Ops without waiting between retries.
func (o *Ops) Until(ctx context.Context, _ time.Duration, f retry.Func) error {
	var minorErr error
	attempts := 0

	for {
		attempts++
		if attempts > o.MaxAttempts {
			return retry.NewError(fmt.Errorf("max attempts reached"), minorErr)
		}

		done, err := f(ctx)
		if err != nil {
			if done {
				return err
			}

			minorErr = err
		} else if done {
			return nil
		}
	}
}

// UntilTimeout implements retry.Ops without waiting between retries. UntilTimeout ignores the timeout
// parameter and instead uses Ops.MaxAttempts to configure, how often f is retried.
func (o *Ops) UntilTimeout(ctx context.Context, interval, _ time.Duration, f retry.Func) error {
	return o.Until(ctx, interval, f)
}
