package kuzco

import "testing"

func TestNewDefaultEmbedOpts(t *testing.T) {
	l := New(nil)
	if l == nil {
		t.Fatal("New returned nil")
	}
	if l.embed.truncate != nil {
		t.Errorf("truncate: want nil, got %v", *l.embed.truncate)
	}
	if l.embed.truncateDirection != "" {
		t.Errorf("truncateDirection: want \"\", got %q", l.embed.truncateDirection)
	}
	if l.embed.dimension != 0 {
		t.Errorf("dimension: want 0, got %d", l.embed.dimension)
	}
}

func TestWithEmbeddingTruncate(t *testing.T) {
	for _, v := range []bool{true, false} {
		l := New(nil, WithEmbeddingTruncate(v))
		if l.embed.truncate == nil {
			t.Fatalf("WithEmbeddingTruncate(%v): want non-nil pointer, got nil", v)
		}
		if *l.embed.truncate != v {
			t.Errorf("WithEmbeddingTruncate(%v): want %v, got %v", v, v, *l.embed.truncate)
		}
	}
}

func TestWithEmbeddingTruncateDirection(t *testing.T) {
	tests := []struct {
		name string
		in   TruncateDirection
		want TruncateDirection
	}{
		{"right", TruncateRight, TruncateRight},
		{"left", TruncateLeft, TruncateLeft},
		{"invalid string", "up", ""},
		{"empty", "", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			l := New(nil, WithEmbeddingTruncateDirection(tc.in))
			if l.embed.truncateDirection != tc.want {
				t.Errorf("want %q, got %q", tc.want, l.embed.truncateDirection)
			}
		})
	}
}

func TestWithEmbeddingDimension(t *testing.T) {
	tests := []struct {
		name string
		in   int
		want int
	}{
		{"positive", 256, 256},
		{"zero", 0, 0},
		{"negative", -1, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			l := New(nil, WithEmbeddingDimension(tc.in))
			if l.embed.dimension != tc.want {
				t.Errorf("want %d, got %d", tc.want, l.embed.dimension)
			}
		})
	}
}
