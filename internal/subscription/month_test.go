package subscription

import "testing"

func TestParseMonth(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{name: "valid", input: "07-2025", want: "07-2025"},
		{name: "invalid month", input: "13-2025", wantErr: true},
		{name: "invalid format", input: "2025-07", wantErr: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseMonth(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if FormatMonth(got) != tc.want {
				t.Fatalf("got %s, want %s", FormatMonth(got), tc.want)
			}
		})
	}
}

func TestMonthsInclusive(t *testing.T) {
	start, err := ParseMonth("11-2025")
	if err != nil {
		t.Fatal(err)
	}
	end, err := ParseMonth("02-2026")
	if err != nil {
		t.Fatal(err)
	}

	if got := MonthsInclusive(start, end); got != 4 {
		t.Fatalf("got %d, want 4", got)
	}
}
