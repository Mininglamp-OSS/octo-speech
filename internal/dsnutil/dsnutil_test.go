package dsnutil

import "testing"

func TestEnsureDSNTimeParams(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "no params — both added",
			in:   "user:pass@tcp(127.0.0.1:3306)/dbname",
			want: "user:pass@tcp(127.0.0.1:3306)/dbname?loc=Local&parseTime=true",
		},
		{
			name: "ParseTime=false wrong case — overridden to parseTime=true",
			in:   "user:pass@tcp(host)/db?ParseTime=false",
			want: "user:pass@tcp(host:3306)/db?loc=Local&parseTime=true",
		},
		{
			name: "both already correct — preserved",
			in:   "user:pass@tcp(host)/db?parseTime=true&loc=Local",
			want: "user:pass@tcp(host:3306)/db?loc=Local&parseTime=true",
		},
		{
			name: "loc=UTC explicit — preserved",
			in:   "user:pass@tcp(host)/db?loc=UTC",
			want: "user:pass@tcp(host:3306)/db?parseTime=true",
		},
		{
			name: "loc=Asia/Shanghai explicit — preserved",
			in:   "user:pass@tcp(host)/db?loc=Asia%2FShanghai",
			want: "user:pass@tcp(host:3306)/db?loc=Asia%2FShanghai&parseTime=true",
		},
		{
			name: "other params — preserved",
			in:   "user:pass@tcp(host)/db?charset=utf8mb4&timeout=5s",
			want: "user:pass@tcp(host:3306)/db?loc=Local&parseTime=true&timeout=5s&charset=utf8mb4",
		},
		{
			name: "empty DSN — empty returned",
			in:   "",
			want: "",
		},
		{
			name: "malformed DSN — returned as-is",
			in:   "not-a-valid-dsn://???",
			want: "not-a-valid-dsn://???",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EnsureDSNTimeParams(tt.in)
			if got != tt.want {
				t.Errorf("EnsureDSNTimeParams(%q)\n got: %q\nwant: %q", tt.in, got, tt.want)
			}
		})
	}
}
