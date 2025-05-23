// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package flow

import (
	"context"
	"errors"
	"sync"
	"time"

	"k8s.io/utils/clock"
)

type progressReporterDelaying struct {
	lock                sync.Mutex
	ctx                 context.Context
	ctxCancel           context.CancelFunc
	reporterFn          ProgressReporterFn
	period              time.Duration
	clock               clock.Clock
	timer               clock.Timer
	pendingProgress     *Stats
	delayProgressReport bool
}

// NewDelayingProgressReporter returns a new progress reporter with the given function and the configured period. A
// period of `0` will lead to immediate reports as soon as flow tasks are completed.
func NewDelayingProgressReporter(clock clock.Clock, reporterFn ProgressReporterFn, period time.Duration) ProgressReporter {
	return &progressReporterDelaying{
		clock:      clock,
		reporterFn: reporterFn,
		period:     period,
	}
}

func (p *progressReporterDelaying) Start(ctx context.Context) error {
	p.lock.Lock()
	defer p.lock.Unlock()

	if p.timer != nil {
		return errors.New("progress reporter has already been started")
	}

	// We store the context on the progressReporterDelaying object so that we can call the reporterFn with the original
	// context - otherwise, the final state cannot be reported because the cancel context will already be canceled
	p.ctx = ctx

	if p.period > 0 {
		p.timer = p.clock.NewTimer(p.period)

		ctx, cancel := context.WithCancel(ctx)
		p.ctxCancel = cancel

		go p.run(ctx)
	}

	return nil
}

func (p *progressReporterDelaying) Stop() {
	p.lock.Lock()

	if p.ctxCancel != nil {
		p.ctxCancel()
	}

	p.ctxCancel = nil
	p.timer = nil
	p.lock.Unlock()
	p.report()
}

func (p *progressReporterDelaying) Report(_ context.Context, pendingProgress *Stats) {
	p.lock.Lock()
	defer p.lock.Unlock()

	if p.timer != nil && p.delayProgressReport {
		p.pendingProgress = pendingProgress
		return
	}

	p.reporterFn(p.ctx, pendingProgress)
	p.delayProgressReport = true
}

func (p *progressReporterDelaying) run(ctx context.Context) {
	timer := p.timer
	for timer != nil {
		select {
		case <-timer.C():
			timer.Reset(p.period)
			p.report()

		case <-ctx.Done():
			timer.Stop()
			return
		}
	}
}

func (p *progressReporterDelaying) report() {
	p.lock.Lock()
	defer p.lock.Unlock()

	if p.pendingProgress != nil {
		p.reporterFn(p.ctx, p.pendingProgress)
		p.pendingProgress = nil
	}
}
