// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package dns_test

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/gardener/gardener/pkg/utils/retry"
)

func TestDns(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Botanist Extensions DNS Suite")
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
	return filepath.Join("../", "../", "../", "../", "../", "charts")

}
