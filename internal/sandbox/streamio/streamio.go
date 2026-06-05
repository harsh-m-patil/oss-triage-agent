package streamio

import (
	"bufio"
	"io"
)

// Lines reads r line-by-line and invokes onLine for each line (without trailing newline).
// Empty lines are delivered. A final partial line without a trailing newline is delivered
// before EOF. If onLine is nil, output is discarded.
func Lines(r io.Reader, onLine func(line string)) error {
	if onLine == nil {
		_, err := io.Copy(io.Discard, r)
		return err
	}

	br := bufio.NewReader(r)
	for {
		line, err := br.ReadString('\n')
		if len(line) > 0 {
			if line[len(line)-1] == '\n' {
				line = line[:len(line)-1]
			}
			onLine(line)
		}
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
	}
}
