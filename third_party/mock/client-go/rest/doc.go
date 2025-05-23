// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0
//go:generate mockgen -package rest -destination=mocks.go k8s.io/client-go/rest HTTPClient,Interface

package rest
