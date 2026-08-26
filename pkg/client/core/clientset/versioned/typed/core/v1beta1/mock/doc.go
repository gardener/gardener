// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

//go:generate mockgen -destination=mocks.go -package=mock github.com/gardener/gardener/pkg/client/core/clientset/versioned/typed/core/v1beta1 CoreV1beta1Interface,ShootInterface,SeedInterface

package mock
