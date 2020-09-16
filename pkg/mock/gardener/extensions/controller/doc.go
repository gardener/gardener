// SPDX-FileCopyrightText: 2019 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

//go:generate mockgen -destination=zz_funcs.go -package=controller github.com/gardener/gardener/pkg/mock/gardener/extensions/controller AddToManager
//go:generate mockgen -destination=mocks.go -package=controller github.com/gardener/gardener/extensions/pkg/controller ChartRendererFactory

package controller

import "sigs.k8s.io/controller-runtime/pkg/manager"

// AddToManager allows mocking controller's AddToManager functions.
type AddToManager interface {
	Do(manager.Manager) error
}
