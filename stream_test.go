package stream_test

import (
	"bytes"
	"io"
	"strings"
	"testing"

	stream "github.com/mutility/goa-stream"
)

type doner interface{ Done() }

func TestJSONL(t *testing.T) {
	for _, tt := range []struct {
		name                  string
		stdin, stdout, stderr string
		data                  func(t testing.TB) doner
	}{
		{
			"nil",
			"",
			"",
			"",
			func(t testing.TB) doner { return &Doner{} },
		},
		{
			"non-grpc",
			"",
			"",
			"",
			func(t testing.TB) doner { return nil },
		},
		{
			"send-three",
			`{"a":1}{"a":2}{"a":3}`,
			"",
			"^^^$\n",
			func(t testing.TB) doner {
				return &Sender{
					t:      t,
					want:   []*Payload{{A: 1}, {A: 2}, {A: 3}},
					failAt: -1,
				}
			},
		},
		{
			"send-two-eof",
			`{"a":1}{"a":2}{"a":3}`,
			"",
			"^^#\n",
			func(t testing.TB) doner {
				return &Sender{
					t:      t,
					want:   []*Payload{{A: 1}, {A: 2}},
					failAt: 1,
				}
			},
		},
		{
			"recv-three",
			"",
			`{
  "A": 1
}
{
  "A": 2
}
{
  "A": 3
}
`,
			"vvv#\n",
			func(t testing.TB) doner {
				return &Recver{
					t:      t,
					want:   []*Payload{{A: 1}, {A: 2}, {A: 3}},
					failAt: -1,
				}
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			stdout := &bytes.Buffer{}
			stderr := &bytes.Buffer{}
			stream.SetStd(
				strings.NewReader(tt.stdin),
				stdout, stderr,
			)

			data := tt.data(t)
			stream.JSONL(data, true)
			if data != nil {
				data.Done()
			}

			if got, want := stdout.String(), tt.stdout; got != want {
				t.Errorf("stdout: got %q, want %q", got, want)
			}
			if got, want := stderr.String(), tt.stderr; got != want {
				t.Errorf("stderr: got %q, want %q", got, want)
			}
		})
	}
}

type Payload struct {
	A int
	B string `json:",omitempty"`
}

type Sender struct {
	t      testing.TB
	want   []*Payload
	next   int
	failAt int
}

func (s *Sender) Send(p *Payload) error {
	if s.next >= len(s.want) {
		s.t.Errorf("Unexpected send %v", *p)
		return nil
	}
	if got, want := *p, *s.want[s.next]; got != want {
		s.t.Errorf("Send: got %v, want %v", got, want)
	}
	if s.next == s.failAt {
		s.next++
		return io.EOF
	}
	s.next++
	return nil
}

func (s *Sender) Close() error {
	if s.next < len(s.want) {
		s.t.Errorf("Send: got %v calls, want %v", s.next, len(s.want))
	}
	if s.next == s.failAt {
		s.next++
		return io.EOF
	}
	s.next = len(s.want) + 1
	return nil
}

func (s *Sender) Done() {
	if s.next <= len(s.want) {
		s.t.Errorf("Close() not called")
	}
}

type Recver struct {
	t      testing.TB
	want   []*Payload
	next   int
	failAt int
}

func (s *Recver) Recv() (*Payload, error) {
	if s.next == len(s.want) {
		return nil, io.EOF
	}
	if s.next > len(s.want) {
		s.t.Errorf("Unexpected recv")
	}
	if s.next == s.failAt {
		return nil, io.ErrUnexpectedEOF
	}
	recv := s.want[s.next]
	s.next++
	return recv, nil
}

func (s *Recver) Done() {
	if s.next < len(s.want) {
		s.t.Errorf("Recv: got %v calls, want %v", s.next, len(s.want))
	}
}

type Doner struct{}

func (*Doner) Done() {}
