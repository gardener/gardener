// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
// SPDX-License-Identifier: Apache-2.0

//go:generate mockgen -package mock -destination=mocks.go github.com/gardener/gardener/pkg/client/kubernetes/clientmap ClientMap,ClientSetFactory

package mock
