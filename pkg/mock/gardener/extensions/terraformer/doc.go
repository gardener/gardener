// SPDX-FileCopyrightText: 2019 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

//go:generate mockgen -package=terraformer -destination=mocks.go github.com/gardener/gardener/extensions/pkg/terraformer Terraformer,Initializer,Factory

package terraformer
