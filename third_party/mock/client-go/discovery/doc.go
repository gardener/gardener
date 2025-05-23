// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0
//go:generate mockgen -package discovery -destination=mocks.go k8s.io/client-go/discovery DiscoveryInterface

package discovery
