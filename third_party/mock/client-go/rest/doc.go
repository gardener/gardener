// SPDX-FileCopyrightText: Copyright Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0
//go:generate mockgen -package rest -destination=mocks.go k8s.io/client-go/rest HTTPClient,Interface

package rest
