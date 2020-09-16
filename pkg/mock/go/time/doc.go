// SPDX-FileCopyrightText: 2018 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

//go:generate mockgen -destination=funcs.go -package=time github.com/gardener/gardener/pkg/mock/go/time Now

package time

import "time"

// Now allows mocking time.Now.
type Now interface {
	Do() time.Time
}
