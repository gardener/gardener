// SPDX-FileCopyrightText: 2019 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

//go:generate mockgen -package=cmd -destination=mocks.go github.com/gardener/gardener/extensions/pkg/controller/cmd Completer,Option,Flagger

package cmd
