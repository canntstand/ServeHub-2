package main

import (
	"fmt"
	"strings"
)

// isUnsafeDefaultValue is used when preserving/editing older secrets.yml files.
// It is not an input validator: it only identifies known template placeholders.
func isUnsafeDefaultValue(value string) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return true
	}
	for _, marker := range []string{
		"replace_with_",
		"change_this",
		"your_",
		"of_the_bot_you_created",
		"where_ansible_is_installed",
		"_secure_password",
		"hbox_key_pepper",
		"ssh-ed25519 replace_",
	} {
		if strings.Contains(value, marker) {
			return true
		}
	}
	return false
}

func requireNonEmptyFields(values map[string]string, keys []string) error {
	for _, key := range keys {
		if strings.TrimSpace(values[key]) == "" {
			return fmt.Errorf("%s: заполните поле", key)
		}
	}
	return nil
}

func requireBootstrapCredentials(values map[string]string, prefix, label string) error {
	rootKey := prefix + "_root_password"
	sshKey := prefix + "_ssh_password"
	sudoKey := prefix + "_sudo_password"

	if strings.TrimSpace(values[rootKey]) != "" ||
		(strings.TrimSpace(values[sshKey]) != "" && strings.TrimSpace(values[sudoKey]) != "") {
		return nil
	}
	return fmt.Errorf("%s: заполните %s либо одновременно %s и %s", label, rootKey, sshKey, sudoKey)
}

// validateDeploymentValues intentionally checks presence only. Detailed syntax,
// format, length and entropy checks belong to the deployment itself, not the GUI.
func validateDeploymentValues(values map[string]string, option int) error {
	if option < 1 || option > 6 {
		return fmt.Errorf("неизвестный сценарий деплоя: %d", option)
	}

	required := []string{
		"admin_user",
		"admin_password",
		"ssh_private_key",
		"borgmatic_encryption_passphrase",
		"offsite_s3_bucket",
		"offsite_s3_access_key_id",
		"offsite_s3_secret_access_key",
		"vps_public_ip",
		"vps_user",
	}
	if err := requireNonEmptyFields(values, required); err != nil {
		return err
	}

	usesVPS := option == 1 || option == 2 || option == 5 || option == 6
	usesLocal := option >= 1 && option <= 4
	doesBootstrap := option == 1 || option == 3 || option == 5

	if usesLocal {
		localRequired := []string{
			"local_private_ip",
			"local_user",
			"server_name",
			"email",
			"webnames_apikey",
			"postgres_user",
			"postgres_password",
			"nextcloud_redis_pass",
			"secret_vaultwarden_password",
			"hbox_pepper",
		}
		if err := requireNonEmptyFields(values, localRequired); err != nil {
			return err
		}
	}

	if usesVPS {
		vpsRequired := []string{
			"telegram_token",
			"telegram_chat_id",
			"hermes_api_base_url",
			"hermes_api_key",
			"hermes_model",
		}
		if err := requireNonEmptyFields(values, vpsRequired); err != nil {
			return err
		}
	}

	if doesBootstrap {
		if option == 1 || option == 5 {
			if strings.TrimSpace(values["ssh_public_key"]) == "" {
				return fmt.Errorf("ssh_public_key: заполните поле")
			}
			if err := requireBootstrapCredentials(values, "vps", "VPS"); err != nil {
				return err
			}
		}
		if option == 1 || option == 3 {
			if strings.TrimSpace(values["ssh_public_key"]) == "" {
				return fmt.Errorf("ssh_public_key: заполните поле")
			}
			if err := requireBootstrapCredentials(values, "local", "домашний сервер"); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateDeploymentSource(updates map[string]string, option int) error {
	_, document, _, _, err := loadSecretsSource()
	if err != nil {
		return err
	}
	if err := mergeSecretUpdates(document, updates); err != nil {
		return err
	}
	return validateDeploymentValues(secretScalarValues(document), option)
}

func (a *App) ValidateDeploymentSecrets(updates map[string]string, option int) CheckResult {
	if err := validateDeploymentSource(updates, option); err != nil {
		return CheckResult{Success: false, Message: err.Error()}
	}
	return CheckResult{Success: true, Message: "Обязательные поля выбранного сценария заполнены."}
}

func validateSavedSecretsForDeployment(option int) error {
	return validateDeploymentSource(nil, option)
}
