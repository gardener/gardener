// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

//go:generate mockgen -destination=mocks.go -package=controlplane github.com/gardener/gardener/extensions/pkg/controller/controlplane Actuator

package controlplane
