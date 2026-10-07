package files

import "testing"

func TestByteRange(t *testing.T) {
	for header, want := range map[string][2]int64{
		"":                            {0, 0},
		"bytes=-1048576":              {-1048576, 0},
		"bytes=100-":                  {100, 0},
		"bytes=100-199":               {100, 100},
		"bytes=0-0":                   {0, 1},
		"bytes=-0":                    {0, 0},
		"bytes=5-4":                   {0, 0},
		"bytes=-5-":                   {0, 0},
		"bytes=0-1,5-6":               {0, 0},
		"items=0-5":                   {0, 0},
		"bytes=0-9223372036854775807": {0, 0},
	} {
		if offset, limit := byteRange(header); offset != want[0] || limit != want[1] {
			t.Errorf("byteRange(%q) = %d, %d, want %v", header, offset, limit, want)
		}
	}
}
