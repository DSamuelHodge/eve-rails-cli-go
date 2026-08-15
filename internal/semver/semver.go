package semver

import (
	"fmt"
	"strconv"
	"strings"
)

// Semver is a strict major.minor.patch version.
type Semver struct {
	Major uint64
	Minor uint64
	Patch uint64
}

// Parse parses a strict x.y.z version string.
func Parse(value string) (Semver, error) {
	parts := strings.Split(value, ".")
	if len(parts) != 3 {
		return Semver{}, fmt.Errorf("invalid version '%s'", value)
	}
	var result Semver
	var err error
	if result.Major, err = parseUint(parts[0]); err != nil {
		return Semver{}, err
	}
	if result.Minor, err = parseUint(parts[1]); err != nil {
		return Semver{}, err
	}
	if result.Patch, err = parseUint(parts[2]); err != nil {
		return Semver{}, err
	}
	return result, nil
}

func parseUint(value string) (uint64, error) {
	parsed, err := strconv.ParseUint(value, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid version component '%s'", value)
	}
	return parsed, nil
}

// Equal reports version equality.
func (s Semver) Equal(other Semver) bool {
	return s.Major == other.Major && s.Minor == other.Minor && s.Patch == other.Patch
}

// String returns the x.y.z representation.
func (s Semver) String() string {
	return fmt.Sprintf("%d.%d.%d", s.Major, s.Minor, s.Patch)
}

// Compare returns -1, 0, or 1 comparing s to other.
func (s Semver) Compare(other Semver) int {
	if s.Major != other.Major {
		if s.Major < other.Major {
			return -1
		}
		return 1
	}
	if s.Minor != other.Minor {
		if s.Minor < other.Minor {
			return -1
		}
		return 1
	}
	if s.Patch != other.Patch {
		if s.Patch < other.Patch {
			return -1
		}
		return 1
	}
	return 0
}
