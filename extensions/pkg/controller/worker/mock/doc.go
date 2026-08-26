// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

//go:generate mockgen -destination=mocks.go -package=mock github.com/gardener/gardener/extensions/pkg/controller/worker Actuator

package mock
