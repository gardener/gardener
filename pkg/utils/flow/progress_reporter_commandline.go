// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package flow

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"sync"
	"text/template"

	"github.com/fatih/color"

	"github.com/gardener/gardener/pkg/utils/signals"
)

const infoTemplate = `
Flow: {{ .stat.FlowName }}
{{ green (printf "Succeeded: %d" (.stat.Succeeded | len ))}} | Failed: {{ .stat.Failed | len }} | Pending: {{.stat.Pending | len }} | Running: {{ .stat.Running | len }}
Running Tasks: 
{{- range $id, $empty := .stat.Running }}
== {{ $id }}
{{- with index $.lastErrs $id }}
{{ red "Last Error:" }} {{ . }}
{{- end }}
{{- end }}

`

var infoTpl = template.Must(template.New("").
	Option("missingkey=zero").
	Funcs(map[string]interface{}{
		"red":   color.RedString,
		"green": color.GreenString,
	}).
	Parse(infoTemplate))

// NewCommandLineProgressReporter returns a new progress reporter that writes the current status of the flow to the
// given output, when a SIGINFO is received (usually Ctrl+T). Any TaskFn wrapped with .RetryUntilTimeout() will have its
// retry errors surfaced here.
func NewCommandLineProgressReporter(out io.Writer) ProgressReporter {
	if out == nil {
		out = os.Stdout
	}
	return &progressReporterCommandline{
		out:      out,
		lastErrs: make(map[TaskID]error),
	}
}

type progressReporterCommandline struct {
	lock      sync.Mutex
	ctxCancel context.CancelFunc
	lastStats *Stats

	out      io.Writer
	lastErrs map[TaskID]error

	// signalChan allows tests to inject a channel to trigger printing
	// without relying on actual OS signals.
	signalChan chan os.Signal
}

func (p *progressReporterCommandline) Start(ctx context.Context) error {
	p.lock.Lock()
	defer p.lock.Unlock()

	ctx, cancel := context.WithCancel(ctx)
	p.ctxCancel = cancel

	var sig <-chan os.Signal
	if p.signalChan != nil {
		sig = p.signalChan
	} else {
		c := make(chan os.Signal, 1)
		signal.Notify(c, signals.Info()...)
		sig = c
	}
	go func() {
		for {
			select {
			case <-sig:
				p.printInfo()
			case <-ctx.Done():
				return
			}
		}
	}()

	return nil
}

func (p *progressReporterCommandline) Stop() {
	p.lock.Lock()
	defer p.lock.Unlock()

	if p.ctxCancel != nil {
		p.ctxCancel()
	}
}

func (p *progressReporterCommandline) Report(_ context.Context, stats *Stats) {
	p.lock.Lock()
	defer p.lock.Unlock()
	p.lastStats = stats
}

func (p *progressReporterCommandline) ReportRetry(_ context.Context, id TaskID, err error) {
	p.lock.Lock()
	defer p.lock.Unlock()
	p.lastErrs[id] = err
}

func (p *progressReporterCommandline) printInfo() {
	if err := infoTpl.Execute(p.out, map[string]interface{}{
		"stat":     p.lastStats,
		"lastErrs": p.lastErrs,
	}); err != nil {
		fmt.Fprintf(p.out, "Failed to print progress: %v", err)
	}
}
