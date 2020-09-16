// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

//go:generate mockgen -package component -destination=mocks.go github.com/gardener/gardener/pkg/operation/botanist/component Deployer,Waiter,DeployWaiter

package component
