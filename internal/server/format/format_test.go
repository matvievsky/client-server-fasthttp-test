package format

import "testing"

func TestBytes(t *testing.T) {
	tests := []struct {
		name string
		in   int64
		want string
	}{
		{name: "bytes", in: 999, want: "999 B"},
		{name: "one-kib", in: 1024, want: "1.00 KiB"},
		{name: "mib", in: 5 * 1024 * 1024, want: "5.00 MiB"},
		{name: "gib-half", in: 1536 * 1024 * 1024, want: "1.50 GiB"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := Bytes(tc.in)
			if got != tc.want {
				t.Fatalf("unexpected format: got %q want %q", got, tc.want)
			}
		})
	}
}

func TestBytesPerSecond(t *testing.T) {
	tests := []struct {
		name string
		in   float64
		want string
	}{
		{name: "bytes-per-second", in: 900, want: "900.00 B/s"},
		{name: "kibibytes-per-second", in: 2048, want: "2.00 KiB/s"},
		{name: "mebibytes-per-second", in: 5 * 1024 * 1024, want: "5.00 MiB/s"},
		{name: "gibibytes-per-second", in: 1.5 * 1024 * 1024 * 1024, want: "1.50 GiB/s"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := BytesPerSecond(tc.in)
			if got != tc.want {
				t.Fatalf("unexpected format: got %q want %q", got, tc.want)
			}
		})
	}
}
