package main

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func setupSecretsTestProject(t *testing.T) string {
	t.Helper()
	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(oldDir)
	})
	varsDir := filepath.Join(projectInstallDirName, "ansible", "vars")
	if err := os.MkdirAll(varsDir, 0700); err != nil {
		t.Fatal(err)
	}
	return varsDir
}

func writeTestFile(t *testing.T, path, content string, mode os.FileMode) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), mode); err != nil {
		t.Fatal(err)
	}
}

func yamlInput(lines ...string) []byte {
	return []byte(strings.Join(lines, string(rune(10))))
}

func TestMergeSecretUpdatesPreservesUnknownFieldsCommentsAndTypes(t *testing.T) {
	source := yamlInput(
		"# retain this file comment",
		"server_name: old.example # retain this inline comment",
		"telegram_chat_id: -1001234567890",
		"feature_enabled: true",
		"custom_numeric: 42",
		"custom_nested:",
		"  token: unquoted-value",
		"  retry_count: 3",
		"ssh_private_key: |",
		"  private-key-line-one",
		"  private-key-line-two",
	)
	document, err := parseSecretsYAML(source)
	if err != nil {
		t.Fatalf("parse YAML: %v", err)
	}
	err = mergeSecretUpdates(document, map[string]string{
		"server_name":     "new.example",
		"vps_ssh_password": "ssh password",
	})
	if err != nil {
		t.Fatalf("merge updates: %v", err)
	}
	serialized, err := marshalSecretsYAML(document)
	if err != nil {
		t.Fatalf("marshal YAML: %v", err)
	}
	if !strings.Contains(string(serialized), "retain this file comment") ||
		!strings.Contains(string(serialized), "retain this inline comment") {
		t.Fatalf("comments were not preserved: %s", serialized)
	}

	reparsed, err := parseSecretsYAML(serialized)
	if err != nil {
		t.Fatalf("reparse YAML: %v", err)
	}
	values := secretScalarValues(reparsed)
	if values["server_name"] != "new.example" {
		t.Fatalf("server_name = %q", values["server_name"])
	}
	if values["custom_numeric"] != "42" || values["custom_nested"] != "" {
		t.Fatalf("unknown scalar/nested fields lost: %#v", values)
	}
	wantPrivateKey := strings.Join([]string{"private-key-line-one", "private-key-line-two", ""}, string(rune(10)))
	if values["ssh_private_key"] != wantPrivateKey {
		t.Fatalf("multiline scalar was not decoded correctly: %q", values["ssh_private_key"])
	}
	root := reparsed.Content[0]
	tags := make(map[string]string)
	for i := 0; i+1 < len(root.Content); i += 2 {
		key, value := root.Content[i].Value, root.Content[i+1]
		tags[key] = value.Tag
	}
	if tags["telegram_chat_id"] != "!!int" {
		t.Fatalf("untouched integer type changed to %s", tags["telegram_chat_id"])
	}
	if tags["feature_enabled"] != "!!bool" {
		t.Fatalf("untouched boolean type changed to %s", tags["feature_enabled"])
	}
	if values["vps_ssh_password"] != "ssh password" {
		t.Fatalf("new SSH password not merged: %#v", values)
	}
}

func TestParseSecretsSupportsUnquotedAndNumericScalars(t *testing.T) {
	document, err := parseSecretsYAML(yamlInput(
		"server_name: example.org",
		"telegram_chat_id: -100123",
		"enabled: true",
		"null_value: null",
	))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	values := secretScalarValues(document)
	for key, want := range map[string]string{
		"server_name":      "example.org",
		"telegram_chat_id": "-100123",
		"enabled":          "true",
		"null_value":       "",
	} {
		if values[key] != want {
			t.Errorf("%s = %q, want %q", key, values[key], want)
		}
	}
}

func TestMergeSecretUpdatesDeletesOnlyBlankOptionalValues(t *testing.T) {
	document, err := parseSecretsYAML(yamlInput(
		"server_name: example.org",
		"adguard_password_salt: keep-me",
		"borgmatic_version: 2.1.10",
		"custom_value: keep-custom",
	))
	if err != nil {
		t.Fatal(err)
	}
	if err := mergeSecretUpdates(document, map[string]string{
		"adguard_password_salt": "",
		"borgmatic_version":     "2.2.0",
		"server_name":           "new.example.org",
	}); err != nil {
		t.Fatal(err)
	}
	content, err := marshalSecretsYAML(document)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := parseSecretsYAML(content)
	if err != nil {
		t.Fatal(err)
	}
	values := secretScalarValues(parsed)
	if _, exists := values["adguard_password_salt"]; exists {
		t.Fatal("blank optional adguard salt should be removed")
	}
	if values["borgmatic_version"] != "2.2.0" || values["custom_value"] != "keep-custom" {
		t.Fatalf("optional or unknown value was lost: %#v", values)
	}
}

func TestMergeRejectsUnknownUpdateKeysAndPreservesUnknownConfigKeys(t *testing.T) {
	document, err := parseSecretsYAML(yamlInput("server_name: example.org", "custom_key: retained"))
	if err != nil {
		t.Fatal(err)
	}
	if err := mergeSecretUpdates(document, map[string]string{"server_name": "new.example.org"}); err != nil {
		t.Fatal(err)
	}
	if err := mergeSecretUpdates(document, map[string]string{"not_a_form_field": "value"}); err == nil {
		t.Fatal("unknown update key should be rejected")
	}
	values := secretScalarValues(document)
	if values["custom_key"] != "retained" {
		t.Fatalf("unknown existing key was lost: %#v", values)
	}
}

func TestParseSecretsRejectsDuplicateKeysAndMultipleDocuments(t *testing.T) {
	cases := [][]string{
		{"server_name: first", "server_name: second"},
		{"server_name: first", "---", "server_name: second"},
		{"server_name: example.org", "nested:", "  enabled: true", "  enabled: false"},
		{"- not-a-mapping"},
	}
	for _, lines := range cases {
		if _, err := parseSecretsYAML(yamlInput(lines...)); err == nil {
			t.Errorf("expected YAML rejection for %q", strings.Join(lines, string(rune(10))))
		}
	}
}

func TestPreviewSecretsPreservesConfigurationAndUsesTemplateWhenMissing(t *testing.T) {
	varsDir := setupSecretsTestProject(t)
	template := strings.Join([]string{
		"server_name: example.org",
		"telegram_chat_id: -100123",
		"custom_config:",
		"  feature: true",
	}, string(rune(10)))
	writeTestFile(t, filepath.Join(varsDir, "secrets.yml.example"), template, 0600)

	preview := (&App{}).PreviewSecrets(map[string]string{"server_name": "changed.example.org"})
	if !preview.Success {
		t.Fatalf("preview failed: %s", preview.Message)
	}
	doc, err := parseSecretsYAML([]byte(preview.Content))
	if err != nil {
		t.Fatal(err)
	}
	values := secretScalarValues(doc)
	if values["server_name"] != "changed.example.org" {
		t.Fatalf("update missing: %#v", values)
	}
	if values["telegram_chat_id"] != "-100123" {
		t.Fatalf("numeric scalar lost: %#v", values)
	}
	if !strings.Contains(preview.Content, "custom_config:") || !strings.Contains(preview.Content, "feature: true") {
		t.Fatalf("unknown nested config was lost: %s", preview.Content)
	}
}

func TestSaveSecretsValidatesReadbackAndSets0600(t *testing.T) {
	varsDir := setupSecretsTestProject(t)
	template := strings.Join([]string{
		"# preserve custom settings",
		"server_name: template.example",
		"telegram_chat_id: -100123",
		"custom_setting:",
		"  nested: yes",
	}, string(rune(10)))
	writeTestFile(t, filepath.Join(varsDir, "secrets.yml.example"), template, 0600)

	app := &App{}
	updates := map[string]string{"server_name": "example.org"}
	preview := app.PreviewSecrets(updates)
	if !preview.Success {
		t.Fatalf("preview failed: %s", preview.Message)
	}
	saved := app.SaveSecrets(updates, preview.Content)
	if !saved.Success {
		t.Fatalf("save failed: %s", saved.Message)
	}

	path := filepath.Join(varsDir, "secrets.yml")
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0600 {
		t.Fatalf("permissions = %04o, want 0600", info.Mode().Perm())
	}
	readBack, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := parseSecretsYAML(readBack); err != nil {
		t.Fatalf("saved YAML failed readback parsing: %v", err)
	}
	if !strings.Contains(string(readBack), "custom_setting:") {
		t.Fatalf("custom setting missing after save: %s", readBack)
	}

	before := string(readBack)
	invalidPreview := strings.Join([]string{"server_name: first", "server_name: second"}, string(rune(10)))
	invalid := app.SaveSecrets(map[string]string{"server_name": "other.example"}, invalidPreview)
	if invalid.Success {
		t.Fatal("invalid preview YAML should not be saved")
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != before {
		t.Fatalf("invalid save changed existing file: before=%s after=%s", before, after)
	}
}

func TestLoadSecretsReturnsYAMLValuesInsteadOfRegexParsing(t *testing.T) {
	varsDir := setupSecretsTestProject(t)
	writeTestFile(t, filepath.Join(varsDir, "secrets.yml"), strings.Join([]string{
		"server_name: example.org",
		"telegram_chat_id: -100123",
		"ssh_private_key: |",
		"  line-one",
		"  line-two",
		"custom_setting: unquoted-value",
	}, string(rune(10))), 0600)
	result := (&App{}).LoadSecrets()
	if !result.Success || result.IsTemplate {
		t.Fatalf("unexpected load result: %#v", result)
	}
	wantPrivateKey := strings.Join([]string{"line-one", "line-two", ""}, string(rune(10)))
	for key, want := range map[string]string{
		"server_name":      "example.org",
		"telegram_chat_id": "-100123",
		"ssh_private_key":  wantPrivateKey,
		"custom_setting":   "unquoted-value",
	} {
		if result.Values[key] != want {
			t.Errorf("%s = %q, want %q", key, result.Values[key], want)
		}
	}
}

func TestTemplateFallbackIsValidatedAndMarked(t *testing.T) {
	varsDir := setupSecretsTestProject(t)
	writeTestFile(t, filepath.Join(varsDir, "secrets.yml.example"), "server_name: example.org" + string(rune(10)) + "custom_setting: retained", 0600)
	result := (&App{}).LoadSecrets()
	if !result.Success || !result.IsTemplate {
		t.Fatalf("expected validated template fallback, got %#v", result)
	}
	if result.Values["custom_setting"] != "retained" {
		t.Fatalf("template values missing: %#v", result.Values)
	}
}

func TestYamlNodeSemanticEqualityDetectsTypeChanges(t *testing.T) {
	left, err := parseSecretsYAML(yamlInput("telegram_chat_id: -100123"))
	if err != nil {
		t.Fatal(err)
	}
	quoted := "telegram_chat_id: " + string(rune(34)) + "-100123" + string(rune(34))
	right, err := parseSecretsYAML([]byte(quoted))
	if err != nil {
		t.Fatal(err)
	}
	if secretYAMLEquivalent(left, right) {
		t.Fatal("integer and string YAML nodes must not compare equal")
	}
}

func TestSecretFormFieldNamesAreUnique(t *testing.T) {
	for _, required := range []string{
		"vps_ssh_password", "vps_sudo_password",
		"local_ssh_password", "local_sudo_password",
		"adguard_password_salt", "borgmatic_version",
	} {
		if _, exists := editableSecretFields[required]; !exists {
			t.Errorf("missing supported form field %s", required)
		}
	}
	var node yaml.Node
	if err := yaml.Unmarshal([]byte("value: unquoted"), &node); err != nil {
		t.Fatal(err)
	}
}

func TestEditableSecretFieldsCoverExampleAndFrontend(t *testing.T) {
	template, err := os.ReadFile(filepath.Join("..", "ansible", "vars", "secrets.yml.example"))
	if err != nil {
		t.Fatalf("read secrets example: %v", err)
	}
	document, err := parseSecretsYAML(template)
	if err != nil {
		t.Fatalf("parse secrets example: %v", err)
	}
	for i := 0; i+1 < len(document.Content[0].Content); i += 2 {
		key := document.Content[0].Content[i].Value
		if _, exists := editableSecretFields[key]; !exists {
			t.Errorf("supported secrets.yml.example key %q is not registered as an editable field", key)
		}
	}

	js, err := os.ReadFile(filepath.Join("frontend", "src", "main.js"))
	if err != nil {
		t.Fatalf("read frontend mapping: %v", err)
	}
	html, err := os.ReadFile(filepath.Join("frontend", "index.html"))
	if err != nil {
		t.Fatalf("read frontend form: %v", err)
	}
	mappingPattern := regexp.MustCompile("(?m)^[[:space:]]*([a-z_][a-z0-9_]*):[[:space:]]*'([^']+)'")
	matches := mappingPattern.FindAllSubmatch(js, -1)
	formIDs := make(map[string]string, len(matches))
	for _, match := range matches {
		formIDs[string(match[1])] = string(match[2])
	}
	quote := string(rune(34))
	for key := range editableSecretFields {
		fieldID, mapped := formIDs[key]
		if !mapped {
			t.Errorf("supported secret %q is missing from the frontend field map", key)
			continue
		}
		if !strings.Contains(string(html), "id="+quote+fieldID+quote) {
			t.Errorf("mapped frontend control id %q for %q is missing from index.html", fieldID, key)
		}
	}
}

func TestSaveSecretsPreservesUnchangedFieldsAndExplicitlyDeletesOptionalValues(t *testing.T) {
	varsDir := setupSecretsTestProject(t)
	writeTestFile(t, filepath.Join(varsDir, "secrets.yml.example"), strings.Join([]string{
		"server_name: template.example",
		"telegram_chat_id: -10042",
		"template_custom: retained",
	}, string(rune(10))), 0600)
	writeTestFile(t, filepath.Join(varsDir, "secrets.yml"), strings.Join([]string{
		"server_name: old.example",
		"telegram_chat_id: -100123",
		"feature_enabled: true",
		"custom_settings:",
		"  nested: preserved",
		"borgmatic_version: 2.1.10",
	}, string(rune(10))), 0600)

	app := &App{}
	updates := map[string]string{
		"server_name": "new.example",
		"borgmatic_version": "",
	}
	preview := app.PreviewSecrets(updates)
	if !preview.Success {
		t.Fatalf("preview failed: %s", preview.Message)
	}
	saved := app.SaveSecrets(updates, preview.Content)
	if !saved.Success {
		t.Fatalf("save failed: %s", saved.Message)
	}

	content, err := os.ReadFile(filepath.Join(varsDir, "secrets.yml"))
	if err != nil {
		t.Fatal(err)
	}
	document, err := parseSecretsYAML(content)
	if err != nil {
		t.Fatal(err)
	}
	values := secretScalarValues(document)
	for key, want := range map[string]string{
		"server_name": "new.example",
		"telegram_chat_id": "-100123",
		"feature_enabled": "true",
	} {
		if values[key] != want {
			t.Errorf("%s = %q, want %q", key, values[key], want)
		}
	}
	if !strings.Contains(string(content), "custom_settings:") ||
		!strings.Contains(string(content), "nested: preserved") {
		t.Fatalf("custom nested settings were lost: %s", content)
	}
	if _, exists := values["borgmatic_version"]; exists {
		t.Fatal("explicitly cleared optional version should be removed")
	}
}

