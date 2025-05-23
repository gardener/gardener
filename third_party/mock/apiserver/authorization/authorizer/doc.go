// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0
//go:generate mockgen -package authorizer -destination=mocks.go k8s.io/apiserver/pkg/authorization/authorizer Authorizer

package authorizer
