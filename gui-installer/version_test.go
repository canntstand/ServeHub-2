package main

import (
	"strings"
	"testing"
)

func TestPinServeHubGitVersionReplacesHardcodedDev(t *testing.T) {
	input := "tasks:\n" +
		"  - name: checkout\n" +
		"    ansible.builtin.git:\n" +
		"      repo: \"https://github.com/canntstand/ServeHub-2.git\"\n" +
		"      dest: \"/home/{{ deploy_user }}/ServeHub-2\"\n" +
		"      version: dev\n" +
		"      update: true\n"
	got, note, err := pinServeHubGitVersion(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(got, "version: \"{{ servehub_version }}\"") {
		t.Fatalf("Git task was not pinned:\n%s", got)
	}
	if note == "" {
		t.Fatal("expected a compatibility note")
	}
}

func TestPinServeHubGitVersionAddsMissingVersion(t *testing.T) {
	input := "tasks:\n" +
		"  - name: checkout\n" +
		"    ansible.builtin.git:\n" +
		"      repo: \"https://github.com/canntstand/ServeHub-2\"\n" +
		"      dest: \"/home/{{ deploy_user }}/ServeHub-2\"\n" +
		"      clone: yes\n" +
		"    become_user: deploy\n"
	got, note, err := pinServeHubGitVersion(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(got, "      version: \"{{ servehub_version }}\"\n") {
		t.Fatalf("missing version field was not inserted:\n%s", got)
	}
	if note == "" {
		t.Fatal("expected a compatibility note for the old playbook")
	}
}

func TestPinServeHubGitVersionIsIdempotent(t *testing.T) {
	input := "tasks:\n" +
		"  - name: checkout\n" +
		"    ansible.builtin.git:\n" +
		"      repo: \"https://github.com/canntstand/ServeHub-2.git\"\n" +
		"      dest: \"/home/{{ deploy_user }}/ServeHub-2\"\n" +
		"      version: \"{{ servehub_version | default('dev') }}\"\n"
	got, note, err := pinServeHubGitVersion(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != input || note != "" {
		t.Fatalf("already-pinned playbook should remain unchanged, note=%q:\n%s", note, got)
	}
}

func TestPinServeHubGitVersionRejectsUnknownLayout(t *testing.T) {
	input := "tasks:\n  - name: unrelated\n    ansible.builtin.command:\n      cmd: echo hello\n"
	if _, _, err := pinServeHubGitVersion(input); err == nil {
		t.Fatal("expected unsupported playbook layout to be rejected")
	}
}

func TestPinServeHubGitVersionRejectsAmbiguousRepositoryTasks(t *testing.T) {
	task := "  - name: checkout\n" +
		"    ansible.builtin.git:\n" +
		"      repo: \"https://github.com/canntstand/ServeHub-2.git\"\n" +
		"      dest: \"/home/{{ deploy_user }}/ServeHub-2\"\n"
	input := "tasks:\n" + task + task
	if _, _, err := pinServeHubGitVersion(input); err == nil {
		t.Fatal("expected ambiguous repository tasks to be rejected")
	}
}

func TestCommitSHAValidation(t *testing.T) {
	if !isFullCommitSHA(strings.Repeat("a", 40)) {
		t.Fatal("40-character SHA should be accepted")
	}
	if !isInputCommitSHA("deadbeef") {
		t.Fatal("abbreviated hexadecimal SHA should be accepted for resolution")
	}
	if isInputCommitSHA("not-a-sha") || isFullCommitSHA("deadbeef") {
		t.Fatal("invalid or incomplete SHA was accepted as a full commit")
	}
}
