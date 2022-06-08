// Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package care

import (
	"context"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/operation"

	"github.com/sirupsen/logrus"
)

// WebhookRemediation contains required information for shoot webhook remediation.
type WebhookRemediation struct {
	logger                 logrus.FieldLogger
	initializeShootClients ShootClientInit
	shoot                  *gardencorev1beta1.Shoot
}

// NewWebhookRemediation creates a new instance for webhook remediation.
func NewWebhookRemediation(op *operation.Operation, shootClientInit ShootClientInit) *WebhookRemediation {
	return &WebhookRemediation{
		logger:                 op.Logger,
		shoot:                  op.Shoot.GetInfo(),
		initializeShootClients: shootClientInit,
	}
}

// Remediate mutates shoot webhooks not following the best practices documented by Kubernetes.
func (r *WebhookRemediation) Remediate(ctx context.Context) error {
	return nil
}
