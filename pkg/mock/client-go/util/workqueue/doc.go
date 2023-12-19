// SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors
// SPDX-License-Identifier: Apache-2.0

//go:generate mockgen -package workqueue -destination=mocks.go k8s.io/client-go/util/workqueue RateLimitingInterface

package workqueue
