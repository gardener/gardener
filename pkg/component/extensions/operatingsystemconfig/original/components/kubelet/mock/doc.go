// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

//go:generate mockgen -package=kubelet -destination=mocks.go github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/original/components/kubelet ConfigCodec

package kubelet
