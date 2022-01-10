// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package controller

import (
	"context"
	"fmt"
	"time"

	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/rand"
)

// GetOrGenerateIdentity reuses an existing or creates a new random Gardener identity
func (o *operation) GetOrGenerateIdentity(ctx context.Context) error {
	// check for existing identity
	cm := &corev1.ConfigMap{}
	if err := o.getGardenClient().Client().Get(ctx, kutil.Key(metav1.NamespaceSystem, cmNameClusterIdentity), cm); err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("failed to check for existing Gardener Identity: %w", err)
		}
		cm = nil
	}

	var (
		containsIdentity bool
		id               string
	)

	if cm != nil {
		id, containsIdentity = cm.Data[cmDataKeyClusterIdentity]
	}

	if cm == nil || !containsIdentity {
		var letters = []rune("abcdefghijklmnopqrstuvwxyz")

		rand.Seed(time.Now().UnixNano())
		suffix := make([]rune, 4)
		for i := range suffix {
			suffix[i] = letters[rand.Intn(len(letters))]
		}

		id = fmt.Sprintf("landscape-%s", string(suffix))
		o.log.Infof("Using new Gardener identity :%s", id)
	} else {
		o.log.Infof("Using Gardener Identity %s from the config map %s/%s in the garden cluster", id, metav1.NamespaceSystem, cmNameClusterIdentity)
	}

	o.imports.Identity = &id
	return nil
}
