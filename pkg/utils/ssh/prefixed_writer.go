// Copyright 2013-2014 Onsi Fakhouri
// Modifications Copyright 2025 SAP SE or an SAP affiliate company and Gardener contributors

// Copied from https://github.com/onsi/gomega/blob/v1.38.0/gexec/prefixed_writer.go for eliminating unwanted
// dependencies in production code. We don't want to import ginkgo/gomega as it tends to mess with global flags etc.

package ssh

import (
	"io"
	"sync"
)

// PrefixedWriter wraps an io.Writer, emitting the passed in prefix at the beginning of each new line.
// This can be useful when running multiple ssh.Connections concurrently - you can prefix the log output of each
// session by passing in a PrefixedWriter.
// This is used by Connection.WithOutputPrefix.
type PrefixedWriter struct {
	prefix        []byte
	writer        io.Writer
	lock          *sync.Mutex
	atStartOfLine bool
}

// NewPrefixedWriter creates a new PrefixedWriter.
func NewPrefixedWriter(prefix string, writer io.Writer) *PrefixedWriter {
	return &PrefixedWriter{
		prefix:        []byte(prefix),
		writer:        writer,
		lock:          &sync.Mutex{},
		atStartOfLine: true,
	}
}

// Write implements io.Writer.
func (w *PrefixedWriter) Write(b []byte) (int, error) {
	w.lock.Lock()
	defer w.lock.Unlock()

	var toWrite []byte

	for _, c := range b {
		if w.atStartOfLine {
			toWrite = append(toWrite, w.prefix...)
		}

		toWrite = append(toWrite, c)

		w.atStartOfLine = c == '\n'
	}

	_, err := w.writer.Write(toWrite)
	if err != nil {
		return 0, err
	}

	return len(b), nil
}
