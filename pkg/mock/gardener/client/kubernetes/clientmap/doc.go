// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

//go:generate mockgen -package clientmap -destination=mock_clientmap.go github.com/gardener/gardener/pkg/client/kubernetes/clientmap ClientMap
//go:generate mockgen -package clientmap -destination=mock_factory.go github.com/gardener/gardener/pkg/client/kubernetes/clientmap ClientSetFactory

package clientmap
