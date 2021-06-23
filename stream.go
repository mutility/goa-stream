package stream

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"reflect"
	"strings"

	"golang.org/x/sync/errgroup"
)

var (
	inF, outF         *string
	strictF, verboseF *bool
)

func Flags(f *flag.FlagSet) {
	inF = f.String("stream-in", "-", "Stream JSONL-encoded payloads from file")
	verboseF = f.Bool("stream-verbose", false, "Print send/recv summaries (implied by verbose)")
	strictF = f.Bool("stream-in-strict", false, "Reject unknown fields")
	outF = f.String("stream-out", "-", "Stream JSONL-encoded results to file")
}

// JSONL converts streams of JSONL from stdin (or -stream-in) to Send calls, and
// streams of Recv results to JSONL on stdout (or -stream-out).
//
// This assumes that data implements a function like one of the following, for
// some types T and U:
//
//    Send(*T) error AND Close() error
//    OR SendAndClose(*T) error
//    AND/OR Recv() (*U, error)
//
// Types T and U are identified via reflection, and use json marshaling.
func JSONL(data interface{}, verbose bool) error {
	in, inclose := input()
	out, outclose := output()
	defer inclose()
	defer outclose()

	dec := json.NewDecoder(in)
	if strictF != nil && *strictF {
		dec.DisallowUnknownFields()
	}
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	err := reflectJSONL(dec, enc, data, verbose || (verboseF != nil && *verboseF))
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
	}
	return nil
}

func nopClose() error { return nil }

func input() (r io.Reader, close func() error) {
	if inF == nil || *inF == "-" {
		return os.Stdin, nopClose
	}
	if *inF != "" {
		i, err := os.Open(*inF)
		if err == nil {
			return i, i.Close
		}
		fmt.Fprintln(os.Stderr, err.Error())
	}
	return strings.NewReader(""), nopClose
}

func output() (w io.Writer, close func() error) {
	if outF == nil || *outF == "-" {
		return os.Stdout, nopClose
	}
	if *outF != "" {
		o, err := os.Create(*outF)
		if err == nil {
			return o, o.Close
		}
		fmt.Fprintln(os.Stderr, err.Error())
	}
	return io.Discard, nopClose
}

func reflectJSONL(dec *json.Decoder, enc *json.Encoder, data interface{}, verbose bool) error {
	p := func(string) (int, error) { return 0, nil }
	if verbose {
		p = os.Stderr.WriteString
	}
	stm := reflect.ValueOf(data)
	sendandclose := false
	send := stm.MethodByName("Send")
	if !send.IsValid() {
		send = stm.MethodByName("SendAndClose")
		sendandclose = true
	}
	eg := errgroup.Group{}
	if send.IsValid() {
		payt := send.Type().In(0).Elem()
		eg.Go(func() error {
			for {
				pay := reflect.New(payt)
				err := dec.Decode(pay.Interface())
				if err == io.EOF {
					p("$") // end of input
					break
				}
				if err != nil {
					p("?") // bad input
					return fmt.Errorf("input: %w", err)
				}
				p("^") // sent
				out := send.Call([]reflect.Value{pay})
				if out[0].IsValid() && !out[0].IsZero() {
					err := out[0].Interface().(error)
					if err == io.EOF {
						p("#") // send EOF (closed)
						break
					}
					p("!") // send error
					return fmt.Errorf("send: %w", out[0].Interface())
				}
			}
			if !sendandclose {
				close := stm.MethodByName("Close")
				if close.IsValid() {
					out := close.Call(nil)
					if out[0].IsValid() && !out[0].IsZero() {
						return out[0].Interface().(error)
					}
				}
			}
			return nil
		})
	}

	recv := stm.MethodByName("Recv")
	if recv.IsValid() {
		eg.Go(func() error {
			for {
				out := recv.Call(nil)
				if out[1].IsValid() && !out[1].IsZero() {
					err := out[1].Interface().(error)
					if err == io.EOF {
						p("#") // server closed
						return nil
					}
					return err
				}
				p("v") // received
				err := enc.Encode(out[0].Interface())
				if err != nil {
					return err
				}
			}
		})
	}
	defer p("\n")
	return eg.Wait()
}
