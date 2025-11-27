// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package flow

import (
	"context"
	"fmt"
	"html/template"
	"io"
	"os"
	"os/signal"
	"sync"

	"github.com/fatih/color"
)

const infoTemplate = `Flow: {{ .stat.FlowName }}
{{ green (printf "Succeeded: %d" (.stat.Succeeded | len ))}} | Failed: {{ .stat.Failed | len }} | Pending: {{.stat.Pending | len }} | Running: {{ .stat.Running | len }}
Running Tasks: 
{{- range $id, $empty := .stat.Running }}
== {{ $id }}
{{- with index $.lastErrs $id }}
{{ red "Last Error:" }} {{ . }}
{{- end }}
{{- end }}
`

// NewCommandLineProgressReporter returns a new progress reporter that writes the current status of the flow to the
// given output, when a SIGINFO is received (usually Ctrl+T).
//
// If you want insight into the last error of a task, you need to pass the reporter to a RetryableTask.
func NewCommandLineProgressReporter(out io.Writer) TaskRetryReporter {
	if out == nil {
		out = os.Stdout
	}
	return &progressReporterCommandline{
		out:      out,
		template: parseTemplate(),
		lastErrs: make(map[TaskID]error),
	}
}

func parseTemplate() *template.Template {
	t, err := template.New("").
		Option("missingkey=zero").
		Funcs(map[string]interface{}{
			"red":   color.RedString,
			"green": color.GreenString,
		}).
		Parse(infoTemplate)
	if err != nil {
		panic(err)
	}
	return t
}

type progressReporterCommandline struct {
	lock      sync.Mutex
	ctxCancel context.CancelFunc
	lastStats *Stats

	template *template.Template
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
		signal.Notify(c, infoSignals()...)
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
	err := p.template.Execute(p.out, map[string]interface{}{
		"stat":     p.lastStats,
		"lastErrs": p.lastErrs,
	})
	if err != nil {
		fmt.Fprintf(p.out, "Failed to print progress: %v", err)
	}
}
