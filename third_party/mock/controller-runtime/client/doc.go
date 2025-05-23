// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0
//go:generate mockgen -package client -destination=mocks.go sigs.k8s.io/controller-runtime/pkg/client Client,StatusWriter,Reader,Writer,SubResourceClient

package client
