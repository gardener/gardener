// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package shoot

import (
	"fmt"

	"github.com/gardener/gardener/pkg/utils/reconcilescheduler"

	"k8s.io/client-go/tools/cache"
)

type id struct {
	namespace string
	name      string
}

func (i id) String() string {
	return fmt.Sprintf("%s/%s", i.namespace, i.name)
}

func (i id) GetName() string {
	return i.name
}

func (i id) GetNamespace() string {
	return i.namespace
}

func newID(namespace, name string) id {
	return id{namespace, name}
}

func newIDFromString(key string) (id, error) {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return id{}, err
	}
	return newID(namespace, name), nil
}

type shootElement struct {
	seed  id
	shoot id
}

func (e *shootElement) String() string {
	return e.shoot.String()
}

func (e *shootElement) GetID() reconcilescheduler.ID {
	return e.shoot
}

func (e *shootElement) GetParentID() reconcilescheduler.ID {
	return e.seed
}
