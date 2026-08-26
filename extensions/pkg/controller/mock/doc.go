// SPDX-FileCopyrightText: Copyright Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

//go:generate mockgen -destination=zz_funcs.go -package=mock github.com/gardener/gardener/extensions/pkg/controller/mock AddToManager
//go:generate mockgen -destination=mocks.go -package=mock github.com/gardener/gardener/extensions/pkg/controller ChartRendererFactory

package mock

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/manager"
)

// AddToManager allows mocking controller's AddToManager functions.
type AddToManager interface {
	Do(context.Context, manager.Manager) error
}
