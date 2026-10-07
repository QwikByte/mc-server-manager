package server

import (
	"slices"
	"testing"
	"time"

	"github.com/QwikByte/noryx/internal/master/operation"
)

// An action on many servers of a node gets as long as stopping them a few at a time may take.
func TestBulkTimeout(t *testing.T) {
	few := append(slices.Repeat([]string{"n1"}, operation.PerNode), "n2")
	if got := bulkTimeout(few); got != 20*time.Minute {
		t.Errorf("a few servers: %s", got)
	}
	many := append(slices.Repeat([]string{"n1"}, 3*operation.PerNode+1), "n2")
	if got := bulkTimeout(many); got != 4*actionTimeout {
		t.Errorf("many servers: %s", got)
	}
}
