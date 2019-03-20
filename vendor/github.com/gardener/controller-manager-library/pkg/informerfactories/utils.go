/*
 * Copyright 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 *
 */

package informerfactories

import (
	"context"
	"fmt"

	"k8s.io/client-go/tools/cache"
)

type StartInterface interface {
	Start(<-chan struct{})
}

func Start(ctx context.Context, startInterface StartInterface, synched ...cache.InformerSynced) error {
	startInterface.Start(ctx.Done())
	if ok := cache.WaitForCacheSync(ctx.Done(), synched...); !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}
	return nil
}
