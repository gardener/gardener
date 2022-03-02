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

package manager

import (
	"strconv"
	"sync"
	"time"

	"github.com/gardener/gardener/pkg/utils"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// LabelKeyLastRotationStartedTime is a constant for a key of a value on a Secret describing the unix timestamps
	// of when the secret was last rotated.
	LabelKeyLastRotationStartedTime = "last-rotation-started-time"
)

type (
	manager struct {
		lock                   sync.Mutex
		logger                 logrus.FieldLogger
		client                 client.Client
		namespace              string
		lastRotationStartTimes nameToUnixTime
		store                  secretStore
	}

	nameToUnixTime map[string]string

	secretStore map[string]secretInfos
	secretInfos struct {
		current secretInfo
		old     *secretInfo
		bundle  *secretInfo
	}
	secretInfo struct {
		obj                     *corev1.Secret
		dataChecksum            string
		lastRotationStartedTime int64
	}
)

var _ Interface = &manager{}

type secretClass string

const (
	current secretClass = "current"
	old     secretClass = "old"
	bundle  secretClass = "bundle"
)

// New returns a new manager for secrets in a given namespace.
func New(logger logrus.FieldLogger, client client.Client, namespace string, nameToTime map[string]time.Time) Interface {
	lastRotationStartTimes := make(map[string]string)

	for name, time := range nameToTime {
		lastRotationStartTimes[name] = strconv.FormatInt(time.UTC().Unix(), 10)
	}

	return &manager{
		logger:                 logger,
		client:                 client,
		namespace:              namespace,
		store:                  make(secretStore),
		lastRotationStartTimes: lastRotationStartTimes,
	}
}

func (m *manager) addToStore(name string, secret *corev1.Secret, class secretClass) error {
	m.lock.Lock()
	defer m.lock.Unlock()

	info, err := computeSecretInfo(secret)
	if err != nil {
		return err
	}

	secrets := m.store[name]

	switch class {
	case current:
		secrets.current = info
	case old:
		if secrets.old == nil || secrets.old.lastRotationStartedTime < info.lastRotationStartedTime {
			secrets.old = &info
		}
	case bundle:
		secrets.bundle = &info
	}

	m.store[name] = secrets

	return nil
}

func (m *manager) getFromStore(name string) (secretInfos, bool) {
	m.lock.Lock()
	defer m.lock.Unlock()

	secrets, ok := m.store[name]
	return secrets, ok
}

func computeSecretInfo(obj *corev1.Secret) (secretInfo, error) {
	var (
		lastRotationStartTime int64
		err                   error
	)

	if v := obj.Labels[LabelKeyLastRotationStartedTime]; len(v) > 0 {
		lastRotationStartTime, err = strconv.ParseInt(obj.Labels[LabelKeyLastRotationStartedTime], 10, 64)
		if err != nil {
			return secretInfo{}, err
		}
	}

	return secretInfo{
		obj:                     obj,
		dataChecksum:            utils.ComputeSecretChecksum(obj.Data),
		lastRotationStartedTime: lastRotationStartTime,
	}, nil
}
