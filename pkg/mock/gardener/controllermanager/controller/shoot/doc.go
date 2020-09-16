// SPDX-FileCopyrightText: 2018 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

//go:generate mockgen -destination=mocks.go -package=shoot github.com/gardener/gardener/pkg/controllermanager/controller/shoot Cron
//go:generate mockgen -destination=funcs.go -package=shoot github.com/gardener/gardener/pkg/mock/gardener/controllermanager/controller/shoot NewCronWithLocation

package shoot

import (
	"time"

	"github.com/gardener/gardener/pkg/controllermanager/controller/shoot"
)

// NewCronWithLocation allows mocking cron.NewWithLocation.
type NewCronWithLocation interface {
	Do(location *time.Location) shoot.Cron
}
