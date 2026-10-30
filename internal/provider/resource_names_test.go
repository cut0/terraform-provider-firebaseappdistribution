package provider

import "testing"

func TestParseGroupName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		project string
		groupID string
		ok      bool
	}{
		{name: "valid", input: "projects/my-project/groups/qa", project: "my-project", groupID: "qa", ok: true},
		{name: "trailing whitespace", input: "projects/my-project/groups/qa\n", project: "my-project", groupID: "qa", ok: true},
		{name: "member id", input: "projects/my-project/groups/qa/testers/a@example.com", ok: false},
		{name: "missing group", input: "projects/my-project/groups/", ok: false},
		{name: "alias only", input: "qa", ok: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			project, groupID, ok := parseGroupName(tt.input)
			if ok != tt.ok || project != tt.project || groupID != tt.groupID {
				t.Fatalf("parseGroupName(%q) = (%q, %q, %v), want (%q, %q, %v)", tt.input, project, groupID, ok, tt.project, tt.groupID, tt.ok)
			}
		})
	}
}

func TestParseGroupMemberID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		project string
		groupID string
		email   string
		ok      bool
	}{
		{name: "valid", input: "projects/my-project/groups/qa/testers/alice@example.com", project: "my-project", groupID: "qa", email: "alice@example.com", ok: true},
		{name: "group name", input: "projects/my-project/groups/qa", ok: false},
		{name: "missing email", input: "projects/my-project/groups/qa/testers/", ok: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			project, groupID, email, ok := parseGroupMemberID(tt.input)
			if ok != tt.ok || project != tt.project || groupID != tt.groupID || email != tt.email {
				t.Fatalf("parseGroupMemberID(%q) = (%q, %q, %q, %v), want (%q, %q, %q, %v)", tt.input, project, groupID, email, ok, tt.project, tt.groupID, tt.email, tt.ok)
			}
		})
	}
}

func TestGroupMemberID(t *testing.T) {
	t.Parallel()

	got := groupMemberID("my-project", "qa", "alice@example.com")
	want := "projects/my-project/groups/qa/testers/alice@example.com"
	if got != want {
		t.Fatalf("groupMemberID() = %q, want %q", got, want)
	}

	project, groupID, email, ok := parseGroupMemberID(got)
	if !ok || project != "my-project" || groupID != "qa" || email != "alice@example.com" {
		t.Fatalf("parseGroupMemberID(groupMemberID()) round trip failed: (%q, %q, %q, %v)", project, groupID, email, ok)
	}
}

func TestTesterEmail(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		"projects/123456/testers/alice@example.com":   "alice@example.com",
		"projects/my-project/testers/Bob@Example.com": "Bob@Example.com",
		"alice@example.com":                           "alice@example.com",
	}

	for input, want := range tests {
		if got := testerEmail(input); got != want {
			t.Errorf("testerEmail(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestFirstNonEmpty(t *testing.T) {
	t.Parallel()

	if got := firstNonEmpty("", "", "third"); got != "third" {
		t.Fatalf("firstNonEmpty() = %q, want %q", got, "third")
	}
	if got := firstNonEmpty(); got != "" {
		t.Fatalf("firstNonEmpty() = %q, want empty", got)
	}
}
