package repository

import (
	"testing"
	"uuid"
)

func TestShardIndex(t *testing.T) {
	tests := []struct {
		id   string
		want int
	}{
		{id: "00000000-0000-4000-8000-000000000000", want: 0},
		{id: "3fffffff-ffff-4fff-bfff-ffffffffffff", want: 0},
		{id: "40000000-0000-4000-8000-000000000000", want: 1},
		{id: "7fffffff-ffff-4fff-bfff-ffffffffffff", want: 1},
		{id: "80000000-0000-4000-8000-000000000000", want: 2},
		{id: "bfffffff-ffff-4fff-bfff-ffffffffffff", want: 2},
		{id: "c0000000-0000-4000-8000-000000000000", want: 3},
		{id: "ffffffff-ffff-4fff-bfff-ffffffffffff", want: 3},
	}

	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			if got := shardIndex(uuid.MustParse(tt.id)); got != tt.want {
				t.Errorf("shardIndex(%s) = %d, want %d", tt.id, got, tt.want)
			}
		})
	}
}

func TestShardIndex_EveryFirstHexDigit(t *testing.T) {
	const hexDigits = "0123456789abcdef"

	for i, digit := range hexDigits {
		id := uuid.MustParse(string(digit) + "0000000-0000-4000-8000-000000000000")

		if got, want := shardIndex(id), i/4; got != want {
			t.Errorf("shardIndex for first digit %q = %d, want %d", digit, got, want)
		}
	}
}
