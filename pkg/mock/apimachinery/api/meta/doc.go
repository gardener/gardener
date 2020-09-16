// SPDX-FileCopyrightText: 2019 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

//go:generate mockgen -package=meta -destination=mocks.go k8s.io/apimachinery/pkg/api/meta RESTMapper

package meta
