// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package dns_test

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/gardener/gardener/pkg/utils/retry"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestDns(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Dns Suite")
}

var _ retry.Ops = &fakeOps{}

type fakeOps struct{}

// Until implements Ops.
func (o *fakeOps) Until(ctx context.Context, interval time.Duration, f retry.Func) error {
	done, err := f(ctx)
	if err != nil {
		return err
	}

	if !done {
		return fmt.Errorf("not ready")
	}

	return nil
}

// UntilTimeout implements Ops.
func (o *fakeOps) UntilTimeout(ctx context.Context, interval, timeout time.Duration, f retry.Func) error {
	return o.Until(ctx, 0, f)
}

func chartsRoot() string {
	return filepath.Join("../", "../", "../", "../", "charts")

}
