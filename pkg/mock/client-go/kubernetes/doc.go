// SPDX-FileCopyrightText: 2018 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0
//go:generate mockgen -package kubernetes -destination=mocks.go k8s.io/client-go/kubernetes Interface

package kubernetes
