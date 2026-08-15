package semver

import "testing"

func TestParseValidVersions(t *testing.T) {
	valid := []string{"1.0.0", "0.1.0", "2.0.1", "10.20.30"}
	for _, value := range valid {
		version, err := Parse(value)
		if err != nil {
			t.Errorf("expected %q to parse, got error: %v", value, err)
			continue
		}
		if version.String() != value {
			t.Errorf("expected %q round trip, got %q", value, version.String())
		}
	}
}

func TestParseInvalidVersions(t *testing.T) {
	invalid := []string{"", "1.0", "1", "a.b.c", "1.0.0.0", "v1.0.0", "1..0"}
	for _, value := range invalid {
		if _, err := Parse(value); err == nil {
			t.Errorf("expected %q to be rejected", value)
		}
	}
}

func TestCompare(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"1.0.0", "1.0.0", 0},
		{"1.0.0", "1.0.1", -1},
		{"1.0.1", "1.0.0", 1},
		{"1.1.0", "1.0.9", 1},
		{"2.0.0", "1.9.9", 1},
	}
	for _, tc := range cases {
		a, errA := Parse(tc.a)
		b, errB := Parse(tc.b)
		if errA != nil || errB != nil {
			t.Fatalf("failed to parse test versions: %v %v", errA, errB)
		}
		if got := a.Compare(b); got != tc.want {
			t.Errorf("Compare(%s, %s) = %d, want %d", tc.a, tc.b, got, tc.want)
		}
	}
}

func TestEqual(t *testing.T) {
	a, err := Parse("1.2.3")
	if err != nil {
		t.Fatal(err)
	}
	b, err := Parse("1.2.3")
	if err != nil {
		t.Fatal(err)
	}
	if !a.Equal(b) {
		t.Error("expected equal versions to compare equal")
	}
}
