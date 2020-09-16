// SPDX-FileCopyrightText: 2018 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0
//go:generate mockgen -package kubernetes -destination=mocks.go github.com/gardener/gardener/pkg/utils/kubernetes NodeLister

package kubernetes
