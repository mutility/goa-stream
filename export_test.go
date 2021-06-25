package stream

import "io"

// SetStd allows a test to override stdin/stdout/stderr
func SetStd(in io.Reader, out io.Writer, err interface {
	io.Writer
	io.StringWriter
}) {
	stdin = in
	stdout = out
	stderr = err
}
