package logs

import (
	"strings"
	"testing"
	"time"
)

func TestCommand(t *testing.T) {
	s, ctx := newStore(t)
	if err := s.write(ctx, []Entry{{Time: time.Now(), Message: "a"}, {Time: time.Now(), Message: "b"}, {Time: time.Now(), Message: "c"}}, nil); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		args       []string
		want, fail string
	}{
		{args: []string{"-n", "2"}, want: "b,c"},
		{args: []string{"-n", "-5"}, fail: "--lines and --since can't be negative"},
		{args: []string{"--since", "-1h"}, fail: "--lines and --since can't be negative"},
	} {
		var out strings.Builder
		cmd := Command(func() (*Store, error) { return s, nil })
		cmd.SetArgs(tc.args)
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		err := cmd.ExecuteContext(ctx)
		if tc.fail != "" {
			if err == nil || err.Error() != tc.fail {
				t.Errorf("%v: err = %v, want %q", tc.args, err, tc.fail)
			}
			continue
		}
		if err != nil {
			t.Fatalf("%v: %v", tc.args, err)
		}
		var got []string
		for line := range strings.Lines(out.String()) {
			fields := strings.Fields(line)
			got = append(got, fields[len(fields)-1]) // the message
		}
		if strings.Join(got, ",") != tc.want {
			t.Errorf("%v: got %q, want %q", tc.args, out.String(), tc.want)
		}
	}
}
