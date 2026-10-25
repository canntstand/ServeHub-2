package main

import (
	"encoding/hex"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/crypto/ssh"
)

func validTestDeploymentValues(t *testing.T) map[string]string {
	t.Helper()
	priv, pub, err := generateSSHKeyPair()
	if err != nil { t.Fatal(err) }
	random := func() string {
		value, err := randomHexSecret(32)
		if err != nil { t.Fatal(err) }
		return value
	}
	return map[string]string{
		"vps_public_ip": "203.0.113.10",
		"vps_user": "deploy",
		"vps_root_password": "root-password-for-test",
		"vps_ssh_password": "",
		"vps_sudo_password": "",
		"local_private_ip": "192.168.1.10",
		"local_user": "deploy",
		"local_root_password": "local-root-password-for-test",
		"local_ssh_password": "",
		"local_sudo_password": "",
		"server_name": "server.example.org",
		"admin_user": "admin",
		"admin_password": random(),
		"email": "admin@example.org",
		"telegram_token": "123456789:" + strings.Repeat("A", 26) + "abcdefghi",
		"telegram_chat_id": "-1001234567890",
		"webnames_apikey": "webnames-provider-key-test-123456",
		"postgres_user": "postgres_master",
		"postgres_password": random(),
		"nextcloud_redis_pass": random(),
		"secret_vaultwarden_password": random(),
		"hbox_pepper": random(),
		"ssh_public_key": pub,
		"ssh_private_key": priv,
		"borgmatic_encryption_passphrase": random(),
		"backup_disk_uuid": "",
		"offsite_s3_endpoint": "https://s3.twcstorage.ru",
		"offsite_s3_region": "ru-1",
		"offsite_s3_bucket": "my-private-backups",
		"offsite_s3_access_key_id": "test-access-key-id-123456",
		"offsite_s3_secret_access_key": "test-secret-access-key-material-1234567890",
		"offsite_s3_workload_prefix": "workload",
		"offsite_s3_edge_prefix": "edge",
		"hermes_api_base_url": "https://api.example.org/v1",
		"hermes_api_key": "provider-api-key-for-test-123456789",
		"hermes_model": "model-name",
		"hermes_dashboard_basic_auth_secret": random(),
	}
}

func TestGenerateSecretDefaultsUsesCryptographicRandomnessAndSSHKeyPair(t *testing.T) {
	varsDir := setupSecretsTestProject(t)
	template := `admin_password: change_this_to_a_very_strong_password
postgres_password: postgres_master_secure_password
nextcloud_redis_pass: redis_secure_password
secret_vaultwarden_password: vaultwarden_admin_panel_secure_password
hbox_pepper: hbox_key_pepper
borgmatic_encryption_passphrase: ""
hermes_dashboard_basic_auth_secret: replace_with_64_hex_characters
ssh_public_key: "ssh-ed25519 REPLACE_WITH_YOUR_PUBLIC_KEY"
ssh_private_key: |
  REPLACE_WITH_YOUR_PRIVATE_SSH_KEY
hermes_api_key: ""
offsite_s3_access_key_id: ""
`
	writeTestFile(t, filepath.Join(varsDir, "secrets.yml.example"), template, 0600)
	result := (&App{}).GenerateSecretDefaults()
	if !result.Success { t.Fatalf("generation failed: %s", result.Message) }
	for _, key := range generatedSecretFieldNames {
		value := result.Values[key]
		if len(value) != 64 { t.Errorf("%s length = %d, expected 64 hex characters", key, len(value)) }
		if _, err := hex.DecodeString(value); err != nil { t.Errorf("%s is not hex: %v", key, err) }
	}
	if result.Values["hermes_api_key"] != "" || result.Values["offsite_s3_access_key_id"] != "" {
		t.Fatal("external provider credentials must not be generated or fabricated")
	}
	signer, err := ssh.ParsePrivateKey([]byte(result.Values["ssh_private_key"]))
	if err != nil { t.Fatalf("generated private key did not parse: %v", err) }
	parsedPublic, _, _, _, err := ssh.ParseAuthorizedKey([]byte(result.Values["ssh_public_key"]))
	if err != nil { t.Fatalf("generated public key did not parse: %v", err) }
	if !strings.EqualFold(hex.EncodeToString(signer.PublicKey().Marshal()), hex.EncodeToString(parsedPublic.Marshal())) {
		t.Fatal("generated SSH public/private keys do not match")
	}
}

func TestGenerateSecretDefaultsDoesNotRotateExistingSecrets(t *testing.T) {
	varsDir := setupSecretsTestProject(t)
	const stable = "custom-existing-secret-value-long-enough-for-production"
	writeTestFile(t, filepath.Join(varsDir, "secrets.yml"), "admin_password: "+stable+"\nhermes_api_key: provider-key-for-test\n", 0600)
	result := (&App{}).GenerateSecretDefaults()
	if !result.Success { t.Fatal(result.Message) }
	if _, exists := result.Values["admin_password"]; exists { t.Fatal("existing non-placeholder secret was unexpectedly rotated") }
}

func TestValidateDeploymentRequiresValidSSHPairAndS3Configuration(t *testing.T) {
	values := validTestDeploymentValues(t)
	if err := validateDeploymentValues(values, 4); err != nil { t.Fatalf("valid local-only scenario rejected: %v", err) }
	mismatchPrivate, mismatchPublic, err := generateSSHKeyPair()
	if err != nil { t.Fatal(err) }
	values["ssh_private_key"] = mismatchPrivate
	values["ssh_public_key"] = mismatchPublic
	if err := validateDeploymentValues(values, 4); err != nil { t.Fatalf("newly matched pair should pass: %v", err) }
	_, anotherPublic, err := generateSSHKeyPair()
	if err != nil { t.Fatal(err) }
	values["ssh_public_key"] = anotherPublic
	if err := validateDeploymentValues(values, 4); err == nil || !strings.Contains(err.Error(), "разным ключевым парам") {
		t.Fatalf("expected mismatched key error, got %v", err)
	}
	values = validTestDeploymentValues(t)
	values["offsite_s3_secret_access_key"] = ""
	if err := validateDeploymentValues(values, 4); err == nil || !strings.Contains(err.Error(), "offsite_s3_secret_access_key") {
		t.Fatalf("expected missing S3 credential error, got %v", err)
	}
}

func TestValidateDeploymentChecksHermesOnlyForVPSScenarios(t *testing.T) {
	values := validTestDeploymentValues(t)
	values["hermes_api_key"] = ""
	if err := validateDeploymentValues(values, 4); err != nil {
		t.Fatalf("local-only scenario should not require Hermes API credentials: %v", err)
	}
	if err := validateDeploymentValues(values, 6); err == nil || !strings.Contains(err.Error(), "hermes_api_key") {
		t.Fatalf("VPS-only scenario must require Hermes API credentials, got %v", err)
	}
}

func TestValidateDeploymentRequiresBootstrapCredentialsOnlyForFullDeploy(t *testing.T) {
	values := validTestDeploymentValues(t)
	values["vps_root_password"] = ""
	values["vps_ssh_password"] = "login-password"
	values["vps_sudo_password"] = ""
	if err := validateDeploymentValues(values, 2); err != nil {
		t.Fatalf("services-only scenario should not require bootstrap credentials: %v", err)
	}
	if err := validateDeploymentValues(values, 1); err == nil || !strings.Contains(err.Error(), "vps_ssh_password") {
		t.Fatalf("full deploy must require a complete login/sudo path, got %v", err)
	}
	values["vps_sudo_password"] = "sudo-password"
	if err := validateDeploymentValues(values, 1); err != nil {
		t.Fatalf("complete SSH/sudo path should be accepted: %v", err)
	}
}

func TestValidateDeploymentRejectsWeakTemplateSecretsAndInvalidIdentityFields(t *testing.T) {
	values := validTestDeploymentValues(t)
	values["admin_password"] = "change_this_to_a_very_strong_password"
	if err := validateDeploymentValues(values, 4); err == nil || !strings.Contains(err.Error(), "admin_password") {
		t.Fatalf("expected weak default rejection, got %v", err)
	}
	values = validTestDeploymentValues(t)
	values["local_private_ip"] = "not-an-ip"
	if err := validateDeploymentValues(values, 4); err == nil || !strings.Contains(err.Error(), "IPv4") {
		t.Fatalf("expected invalid IP rejection, got %v", err)
	}
	values = validTestDeploymentValues(t)
	values["email"] = "not-an-email"
	if err := validateDeploymentValues(values, 4); err == nil || !strings.Contains(err.Error(), "email") {
		t.Fatalf("expected invalid email rejection, got %v", err)
	}
}

func TestGenerateSecretDefaultsDoesNotFabricateExternalCredentials(t *testing.T) {
	varsDir := setupSecretsTestProject(t)
	writeTestFile(t, filepath.Join(varsDir, "secrets.yml.example"),
		"admin_password: \"\"\nhermes_api_key: \"\"\noffsite_s3_access_key_id: \"\"\noffsite_s3_secret_access_key: \"\"\n", 0600)
	result := (&App{}).GenerateSecretDefaults()
	if !result.Success {
		t.Fatalf("generation failed: %s", result.Message)
	}
	for _, key := range []string{"hermes_api_key", "offsite_s3_access_key_id", "offsite_s3_secret_access_key"} {
		if _, exists := result.Values[key]; exists {
			t.Errorf("external credential %s must not be generated", key)
		}
	}
	if len(result.Values["admin_password"]) != 64 {
		t.Fatal("locally generatable admin password should be generated")
	}
}

func TestS3BucketCanUseExistingServeHubBucketName(t *testing.T) {
	values := validTestDeploymentValues(t)
	values["offsite_s3_bucket"] = "servehub-2-bucket"
	if err := validateDeploymentValues(values, 4); err != nil {
		t.Fatalf("valid existing S3 bucket should not be rejected as a weak secret: %v", err)
	}
}

func TestDeploymentValidationRejectsExampleDomainAndWeakS3Keys(t *testing.T) {
	values := validTestDeploymentValues(t)
	values["server_name"] = "192.168.1.2"
	if err := validateDeploymentValues(values, 4); err == nil || !strings.Contains(err.Error(), "server_name") {
		t.Fatalf("IP address must not be accepted as a DNS domain, got %v", err)
	}
	values = validTestDeploymentValues(t)
	values["offsite_s3_access_key_id"] = "short"
	if err := validateDeploymentValues(values, 4); err == nil || !strings.Contains(err.Error(), "offsite_s3_access_key_id") {
		t.Fatalf("weak S3 access key should be rejected, got %v", err)
	}
}

func TestValidateFullBootstrapRejectsWeakRootPassword(t *testing.T) {
	values := validTestDeploymentValues(t)
	values["vps_root_password"] = "password"
	if err := validateDeploymentValues(values, 1); err == nil || !strings.Contains(err.Error(), "vps_root_password") {
		t.Fatalf("weak root password should be rejected for full bootstrap, got %v", err)
	}
}

func TestValidateDeploymentRequiresOnlyS3PrefixForSelectedTarget(t *testing.T) {
	values := validTestDeploymentValues(t)
	values["offsite_s3_edge_prefix"] = ""
	if err := validateDeploymentValues(values, 4); err != nil {
		t.Fatalf("local-only deployment should not require Edge prefix: %v", err)
	}
	if err := validateDeploymentValues(values, 2); err == nil || !strings.Contains(err.Error(), "offsite_s3_edge_prefix") {
		t.Fatalf("both-server deployment must require Edge prefix: %v", err)
	}
	values = validTestDeploymentValues(t)
	values["offsite_s3_workload_prefix"] = ""
	if err := validateDeploymentValues(values, 6); err != nil {
		t.Fatalf("VPS-only deployment should not require Workload prefix: %v", err)
	}
	if err := validateDeploymentValues(values, 2); err == nil || !strings.Contains(err.Error(), "offsite_s3_workload_prefix") {
		t.Fatalf("both-server deployment must require Workload prefix: %v", err)
	}
}

func TestGenerateSecretDefaultsGeneratesMissingButNotExplicitlyBlankExistingValues(t *testing.T) {
	varsDir := setupSecretsTestProject(t)
	writeTestFile(t, filepath.Join(varsDir, "secrets.yml"),
		"server_name: server.example.org\nadmin_password: \"\"\n", 0600)

	result := (&App{}).GenerateSecretDefaults()
	if !result.Success {
		t.Fatalf("generation failed: %s", result.Message)
	}
	if _, exists := result.Values["admin_password"]; exists {
		t.Fatal("explicitly blank existing secret should not be silently replaced")
	}
	if len(result.Values["postgres_password"]) != 64 {
		t.Fatal("missing password key should receive a generated secure value")
	}
	if len(result.Values["ssh_private_key"]) == 0 || len(result.Values["ssh_public_key"]) == 0 {
		t.Fatal("missing SSH key pair should be generated for an existing config that has no key entries")
	}
}

func TestValidateStrongSecretRejectsRepeatedAndLowEntropyValues(t *testing.T) {
	for _, value := range []string{
		strings.Repeat("a", 64),
		strings.Repeat("abcd", 16),
		"passwordpasswordpasswordpassword",
		"12345678901234567890123456789012",
	} {
		err := validateStrongSecret(map[string]string{"secret": value}, "secret", "secret", 32)
		if err == nil || !strings.Contains(err.Error(), "слишком простой") {
			t.Errorf("expected low-entropy value %q to be rejected, got %v", value, err)
		}
	}
	values := map[string]string{"secret": "8f0b82c5d9a6e1f743c0b8e2d14a7f6c"}
	if err := validateStrongSecret(values, "secret", "secret", 32); err != nil {
		t.Fatalf("diverse 32-character secret should be accepted: %v", err)
	}
}
