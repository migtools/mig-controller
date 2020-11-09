<<<<<<< HEAD
// untested sections: 1

=======
>>>>>>> cbc9bb05... fixup add vendor back
package gexec

import (
	"io"
	"sync"
)

/*
<<<<<<< HEAD
PrefixedWriter wraps an io.Writer, emitting the passed in prefix at the beginning of each new line.
=======
PrefixedWriter wraps an io.Writer, emiting the passed in prefix at the beginning of each new line.
>>>>>>> cbc9bb05... fixup add vendor back
This can be useful when running multiple gexec.Sessions concurrently - you can prefix the log output of each
session by passing in a PrefixedWriter:

gexec.Start(cmd, NewPrefixedWriter("[my-cmd] ", GinkgoWriter), NewPrefixedWriter("[my-cmd] ", GinkgoWriter))
*/
type PrefixedWriter struct {
	prefix        []byte
	writer        io.Writer
	lock          *sync.Mutex
	atStartOfLine bool
}

func NewPrefixedWriter(prefix string, writer io.Writer) *PrefixedWriter {
	return &PrefixedWriter{
		prefix:        []byte(prefix),
		writer:        writer,
		lock:          &sync.Mutex{},
		atStartOfLine: true,
	}
}

func (w *PrefixedWriter) Write(b []byte) (int, error) {
	w.lock.Lock()
	defer w.lock.Unlock()

	toWrite := []byte{}

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
