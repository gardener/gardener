// SPDX-FileCopyrightText: 2019 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

//go:generate mockgen -destination=mocks.go -package=runtime k8s.io/apimachinery/pkg/runtime Decoder

package runtime
