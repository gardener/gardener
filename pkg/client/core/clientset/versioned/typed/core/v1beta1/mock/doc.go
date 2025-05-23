// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

//go:generate mockgen -destination=mocks.go -package=mock github.com/gardener/gardener/pkg/client/core/clientset/versioned/typed/core/v1beta1 CoreV1beta1Interface,ShootInterface,SeedInterface

package mock
