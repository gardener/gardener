// SPDX-FileCopyrightText: 2019 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

//go:generate mockgen -package kubernetes -destination=mocks.go github.com/gardener/gardener/pkg/client/kubernetes Interface
//go:generate mockgen -package kubernetes -destination=mocks_applier.go github.com/gardener/gardener/pkg/client/kubernetes Applier
//go:generate mockgen -package kubernetes -destination=mocks_chartapplier.go github.com/gardener/gardener/pkg/client/kubernetes ChartApplier

package kubernetes
