package terminal

import (
	"errors"
	"strings"
	"unicode"
)

// splitArgs splits a command line into arguments like a shell: spaces separate them and
// quotes keep them together. Nothing is expanded or run by a shell.
func splitArgs(line string) ([]string, error) {
	var (
		args  []string
		arg   strings.Builder
		quote rune
		inArg bool // also true for an empty quoted argument
	)
	for _, r := range line {
		switch {
		case quote != 0 && r == quote:
			quote = 0
		case quote != 0:
			arg.WriteRune(r)
		case r == '"' || r == '\'':
			quote, inArg = r, true
		case unicode.IsSpace(r):
			if inArg {
				args = append(args, arg.String())
				arg.Reset()
				inArg = false
			}
		default:
			arg.WriteRune(r)
			inArg = true
		}
	}
	if quote != 0 {
		return nil, errors.New("a quote is not closed")
	}
	if inArg {
		args = append(args, arg.String())
	}
	return args, nil
}
