// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

//go:generate mockgen -package mock -destination=mocks.go github.com/gardener/gardener/pkg/client/kubernetes Interface
//go:generate mockgen -package mock -destination=mocks_applier.go github.com/gardener/gardener/pkg/client/kubernetes Applier
//go:generate mockgen -package mock -destination=mocks_chartapplier.go github.com/gardener/gardener/pkg/client/kubernetes ChartApplier
//go:generate mockgen -package mock -destination=mocks_podexecutor.go github.com/gardener/gardener/pkg/utils/kubernetes PodExecutor

package mock
