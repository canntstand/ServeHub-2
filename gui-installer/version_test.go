package main

import (
	"os"
	"path/filepath"
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


func writeMinimalProjectForIntegrityTest(t *testing.T, projectDir string) {
	t.Helper()
	required := []string{
		"docker-compose.ansible.yaml",
		"docker-compose.local.yaml",
		"docker-compose.remote.yaml",
		"ansible/deploy.yml",
		"ansible/tasks/bootstrap_os.yml",
		"ansible/tasks/setup_local.yml",
		"ansible/tasks/setup_remote.yml",
		"ansible/vars/secrets.yml.example",
		"scripts/ansible-entrypoint.sh",
	}
	for _, relative := range required {
		fullPath := filepath.Join(projectDir, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(fullPath, []byte("test project content: "+relative+"\n"), 0644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestProjectInstallStateRequiresMatchingIntegrityFingerprint(t *testing.T) {
	projectDir := filepath.Join(t.TempDir(), "ServeHub-2")
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatal(err)
	}
	writeMinimalProjectForIntegrityTest(t, projectDir)

	state := getProjectInstallState(projectDir)
	if !state.Exists || !state.Complete || state.KnownVersion || state.IntegrityVerified {
		t.Fatalf("project without version metadata should not be trusted: %#v", state)
	}

	metadata := ProjectVersionMetadata{
		SchemaVersion: 1,
		SourceMode:    "commit",
		SourceRef:     strings.Repeat("a", 40),
		ResolvedRef:   strings.Repeat("a", 40),
		CommitSHA:     strings.Repeat("a", 40),
		DisplayName:   "test commit",
		InstalledAt:   "2026-01-01T00:00:00Z",
	}
	if err := writeProjectMetadata(projectDir, metadata); err != nil {
		t.Fatal(err)
	}
	state = getProjectInstallState(projectDir)
	if state.KnownVersion || state.IntegrityVerified {
		t.Fatal("legacy metadata without an integrity fingerprint must not prove the installed code version")
	}

	fingerprint, err := projectIntegrityFingerprint(projectDir)
	if err != nil {
		t.Fatal(err)
	}
	metadata.IntegritySHA256 = fingerprint
	if err := writeProjectMetadata(projectDir, metadata); err != nil {
		t.Fatal(err)
	}
	state = getProjectInstallState(projectDir)
	if !state.KnownVersion || !state.IntegrityVerified || !state.Complete {
		t.Fatalf("valid metadata and matching integrity fingerprint should be trusted: %#v", state)
	}

	coreFile := filepath.Join(projectDir, "docker-compose.local.yaml")
	if err := os.WriteFile(coreFile, []byte("unexpected code modification\n"), 0644); err != nil {
		t.Fatal(err)
	}
	state = getProjectInstallState(projectDir)
	if !state.Complete || state.KnownVersion || state.IntegrityVerified {
		t.Fatalf("modified core file must invalidate version verification: %#v", state)
	}
}
