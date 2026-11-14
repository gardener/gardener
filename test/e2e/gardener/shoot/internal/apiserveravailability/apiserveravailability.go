// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package apiserveravailability

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/client-go/rest"

	. "github.com/gardener/gardener/test/e2e/gardener"
)

const (
	// pollInterval is the interval between two consecutive probes of the API server.
	pollInterval = 1 * time.Second
	// requestTimeout is the per-probe timeout.
	requestTimeout = 2 * time.Second
	// consecutiveFailureTolerance is the number of back-to-back failed probes tolerated before they are considered as
	// downtime. It mirrors the seed-side zero-downtime validator (test/utils/shoots/update/highavailability), which
	// tolerates 3 consecutive curl failures to avoid flakiness from transient network hiccups of the probe itself.
	consecutiveFailureTolerance = 3
)

// Probe continuously probes the shoot's external API server endpoint (the advertised address whose DNS record is
// repointed from the source to the destination seed during a live control plane migration) via the admin kubeconfig.
// It runs in the test process so that it is independent of which seed currently hosts the control plane and keeps
// probing across the whole migration window. This complements the point-in-time inclusterclient checks, which only
// prove availability before and after the migration, not during the cutover.
type Probe struct {
	cancel context.CancelFunc
	done   chan struct{}

	mu            sync.Mutex
	total         int
	failures      int
	currentStreak int
	longestStreak int
	lastErr       error
}

// ItShouldStartProbe starts probing the shoot API server in the background. ItShouldInitializeShootClient must have run
// before so that the admin kubeconfig based client set is available.
func (p *Probe) ItShouldStartProbe(s *ShootContext) {
	GinkgoHelper()

	It("Start continuous probe of the shoot API server external endpoint", func(_ SpecContext) {
		Expect(s.ShootClientSet).NotTo(BeNil(), "ItShouldInitializeShootClient should be called first")

		ctx, cancel := context.WithCancel(context.Background())
		p.cancel = cancel
		p.done = make(chan struct{})

		restClient := s.ShootClientSet.RESTClient()

		go func() {
			defer GinkgoRecover()
			defer close(p.done)

			ticker := time.NewTicker(pollInterval)
			defer ticker.Stop()

			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					reqCtx, reqCancel := context.WithTimeout(ctx, requestTimeout)
					err := probeHealthz(reqCtx, restClient)
					reqCancel()

					// Don't count the in-flight probe that was interrupted by the shutdown as a failure.
					if errors.Is(err, context.Canceled) {
						return
					}
					p.record(err)
				}
			}
		}()
	}, SpecTimeout(time.Minute))
}

// ItShouldStopProbeAndEnsureNoDowntime stops the background probe and asserts that the API server did not experience
// sustained downtime during the live migration.
func (p *Probe) ItShouldStopProbeAndEnsureNoDowntime(s *ShootContext) {
	GinkgoHelper()

	It("Stop probe and ensure the shoot API server stayed available during live migration", func(_ SpecContext) {
		Expect(p.cancel).NotTo(BeNil(), "ItShouldStartProbe should be called first")

		p.cancel()
		Eventually(p.done).WithTimeout(time.Minute).Should(BeClosed(), "probe goroutine should terminate")

		p.mu.Lock()
		defer p.mu.Unlock()

		s.Log.Info("Shoot API server availability probe finished",
			"totalProbes", p.total,
			"failedProbes", p.failures,
			"longestConsecutiveFailures", p.longestStreak,
		)

		Expect(p.total).To(BeNumerically(">", 0), "probe should have run at least once")
		Expect(p.longestStreak).To(BeNumerically("<=", consecutiveFailureTolerance),
			"shoot API server was unavailable for %d consecutive probes (~%s) during the live migration; last error: %v",
			p.longestStreak, time.Duration(p.longestStreak)*pollInterval, p.lastErr,
		)
	}, SpecTimeout(2*time.Minute))
}

func (p *Probe) record(err error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.total++
	if err != nil {
		p.failures++
		p.currentStreak++
		if p.currentStreak > p.longestStreak {
			p.longestStreak = p.currentStreak
		}
		p.lastErr = err
		return
	}
	p.currentStreak = 0
}

func probeHealthz(ctx context.Context, restClient rest.Interface) error {
	response := restClient.Get().AbsPath("/healthz").Do(ctx)
	if err := response.Error(); err != nil {
		return err
	}

	var statusCode int
	response.StatusCode(&statusCode)
	if statusCode != http.StatusOK {
		return fmt.Errorf("API server /healthz endpoint returned non-ok status code %d", statusCode)
	}

	return nil
}
