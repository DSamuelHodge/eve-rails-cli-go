package hotload

import (
	"testing"

	"github.com/DSamuelHodge/eve-rails-cli-go/internal/versioning"
)

func TestParseComponentRef(t *testing.T) {
	ref, err := Parse("skill:summarize_thread@1.0.1")
	if err != nil {
		t.Fatal(err)
	}
	if ref.Kind != KindSkill || ref.Name != "summarize_thread" || ref.Version != "1.0.1" {
		t.Fatalf("unexpected ref: %+v", ref)
	}
}

func TestParseRejectsBadRefs(t *testing.T) {
	invalid := []string{"skill", "skill:foo", "unknown:foo@1.0.0", "skill:With Caps@1.0.0"}
	for _, value := range invalid {
		if _, err := Parse(value); err == nil {
			t.Errorf("expected %q to be rejected", value)
		}
	}
}

func TestSkillPatchCanHotload(t *testing.T) {
	ref, err := Parse("skill:summarize@1.0.1")
	if err != nil {
		t.Fatal(err)
	}
	classification, err := Classify(ref, "1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	if classification.Action != ActionHotload {
		t.Errorf("expected hotload, got %s", classification.Action)
	}
}

func TestToolPatchRequiresRedeploy(t *testing.T) {
	ref, err := Parse("tool:search@1.0.1")
	if err != nil {
		t.Fatal(err)
	}
	classification, err := Classify(ref, "1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	if classification.Action != ActionRedeploy {
		t.Errorf("expected redeploy, got %s", classification.Action)
	}
}

func TestMemoryChangeRequiresMigration(t *testing.T) {
	ref, err := Parse("memory:crm@1.1.0")
	if err != nil {
		t.Fatal(err)
	}
	classification, err := Classify(ref, "1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	if classification.Action != ActionMigration {
		t.Errorf("expected migration, got %s", classification.Action)
	}
}

func TestApprovalMajorUpdateCannotHotload(t *testing.T) {
	ref, err := Parse("approval:required@2.0.0")
	if err != nil {
		t.Fatal(err)
	}
	classification, err := Classify(ref, "1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	if classification.Action == ActionHotload {
		t.Error("expected approval major update to not hotload")
	}
	if classification.Action != ActionRedeploy {
		t.Errorf("expected redeploy for major, got %s", classification.Action)
	}
}

func TestClassifyVersionChange(t *testing.T) {
	cases := []struct {
		current, next string
		want          versioning.UpdateKind
	}{
		{"1.0.0", "1.0.1", versioning.UpdatePatch},
		{"1.0.0", "1.1.0", versioning.UpdateMinor},
		{"1.0.0", "2.0.0", versioning.UpdateMajor},
		{"1.0.0", "1.0.0", versioning.UpdateCurrent},
		{"1.0.0", "junk", versioning.UpdateInvalid},
	}
	for _, tc := range cases {
		if got := ClassifyVersionChange(tc.current, tc.next); got != tc.want {
			t.Errorf("ClassifyVersionChange(%s, %s) = %s, want %s", tc.current, tc.next, got, tc.want)
		}
	}
}
