// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

//go:generate mockgen -package=mock -destination=mocks.go github.com/gardener/gardener/extensions/pkg/webhook/cloudprovider Ensurer
package mock
