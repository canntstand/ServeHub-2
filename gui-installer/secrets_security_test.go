package main

import (
	"strings"
	"testing"
)

func validTestDeploymentValues(t *testing.T) map[string]string {
	t.Helper()
	return map[string]string{
		"vps_public_ip": "203.0.113.10",
		"vps_user": "deploy",
		"vps_root_password": "root-password",
		"vps_ssh_password": "",
		"vps_sudo_password": "",
		"local_private_ip": "192.168.1.10",
		"local_user": "deploy",
		"local_root_password": "local-root-password",
		"local_ssh_password": "",
		"local_sudo_password": "",
		"server_name": "server.infra-test.net",
		"admin_password": "x",
		"email": "not-an-email",
		"telegram_token": "x",
		"telegram_chat_id": "not-a-number",
		"webnames_apikey": "x",
		"postgres_password": "x",
		"nextcloud_redis_pass": "x",
		"secret_vaultwarden_password": "x",
		"hbox_pepper": "x",
		"ssh_public_key": "not-a-public-key",
		"ssh_private_key": "not-a-private-key",
		"borgmatic_encryption_passphrase": "x",
		"backup_disk_uuid": "not-a-uuid",
		"offsite_s3_endpoint": "not-a-url",
		"offsite_s3_region": "not a region",
		"offsite_s3_bucket": "***",
		"offsite_s3_access_key_id": "x",
		"offsite_s3_secret_access_key": "x",
		"offsite_s3_workload_prefix": "",
		"offsite_s3_edge_prefix": "",
		"hermes_api_base_url": "not-a-url",
		"hermes_api_key": "x",
		"hermes_model": "x",
	}
}

func TestValidateDeploymentChecksPresenceOnly(t *testing.T) {
	values := validTestDeploymentValues(t)
	if err := validateDeploymentValues(values, 4); err != nil {
		t.Fatalf("non-empty values should not be rejected for format or strength: %v", err)
	}

	for _, key := range []string{
		"admin_password",
		"ssh_private_key",
		"borgmatic_encryption_passphrase",
		"offsite_s3_bucket",
		"offsite_s3_access_key_id",
		"offsite_s3_secret_access_key",
		"vps_public_ip",
		"vps_user",
		"local_private_ip",
		"local_user",
		"server_name",
		"email",
		"webnames_apikey",
		"postgres_password",
		"nextcloud_redis_pass",
		"secret_vaultwarden_password",
		"hbox_pepper",
	} {
		missing := validTestDeploymentValues(t)
		missing[key] = "  "
		err := validateDeploymentValues(missing, 4)
		if err == nil || !strings.Contains(err.Error(), key) {
			t.Errorf("expected only an empty-field error for %q, got %v", key, err)
		}
	}
}

func TestValidateDeploymentChecksHermesOnlyForVPSTargets(t *testing.T) {
	values := validTestDeploymentValues(t)
	values["hermes_api_key"] = ""
	values["telegram_token"] = ""
	values["telegram_chat_id"] = ""
	if err := validateDeploymentValues(values, 4); err != nil {
		t.Fatalf("local-only deployment must not require Edge application credentials: %v", err)
	}
	if err := validateDeploymentValues(values, 6); err == nil || !strings.Contains(err.Error(), "hermes_api_key") {
		t.Fatalf("VPS-only deployment must require a non-empty Hermes API key, got %v", err)
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
		t.Fatalf("full deploy must require root password or both SSH/sudo passwords, got %v", err)
	}
	values["vps_sudo_password"] = "sudo-password"
	if err := validateDeploymentValues(values, 1); err != nil {
		t.Fatalf("non-empty SSH/sudo pair should be accepted without format checks: %v", err)
	}
}

func TestLocalOnlyScenariosRequireVPSEdgeConnection(t *testing.T) {
	for _, option := range []int{3, 4} {
		values := validTestDeploymentValues(t)
		values["hermes_api_key"] = ""
		values["telegram_token"] = ""
		values["telegram_chat_id"] = ""
		if err := validateDeploymentValues(values, option); err != nil {
			t.Fatalf("local-only scenario %d should not require Edge application secrets: %v", option, err)
		}
		values["vps_public_ip"] = ""
		if err := validateDeploymentValues(values, option); err == nil || !strings.Contains(err.Error(), "vps_public_ip") {
			t.Fatalf("local-only scenario %d must require a non-empty Edge address: %v", option, err)
		}
	}
}

func TestServiceOnlyScenariosDoNotRequireBootstrapFields(t *testing.T) {
	for _, option := range []int{2, 4, 6} {
		values := validTestDeploymentValues(t)
		values["vps_root_password"] = ""
		values["vps_ssh_password"] = ""
		values["vps_sudo_password"] = ""
		values["local_root_password"] = ""
		values["local_ssh_password"] = ""
		values["local_sudo_password"] = ""
		values["ssh_public_key"] = ""
		if err := validateDeploymentValues(values, option); err != nil {
			t.Errorf("services-only scenario %d should not require bootstrap credentials/public key: %v", option, err)
		}
	}
}

func TestValidateDeploymentRejectsUnknownScenarioOnly(t *testing.T) {
	values := validTestDeploymentValues(t)
	if err := validateDeploymentValues(values, 0); err == nil {
		t.Fatal("unknown scenario must be rejected")
	}
}
