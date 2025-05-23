// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

//go:generate mockgen -destination=mocks.go -package=mock github.com/gardener/gardener/extensions/pkg/controller/controlplane Actuator

package mock
