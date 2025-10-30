// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

//go:generate mockgen -package workqueue -destination=mocks.go -source=rate_limiting_queue.go TypedRateLimitingInterface
//go:generate mockgen -package workqueue -destination=mocks_rate_limiters.go -source=default_rate_limiters.go TypedRateLimiter

package workqueue
