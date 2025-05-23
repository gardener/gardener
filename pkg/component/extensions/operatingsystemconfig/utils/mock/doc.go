// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

//go:generate mockgen -package=utils -destination=mocks.go github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/utils UnitSerializer,FileContentInlineCodec

package utils
