package main

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"net"
	"net/mail"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"golang.org/x/crypto/ssh"
	"gopkg.in/yaml.v3"
)

var (
	deploymentUsernameRegexp = regexp.MustCompile("^[a-z_][a-z0-9_-]{0,31}$")
	serviceUsernameRegexp = regexp.MustCompile("^[A-Za-z0-9][A-Za-z0-9_.-]{0,63}$")
	deploymentDomainRegexp = regexp.MustCompile("(?i)^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?(?:\\.[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?)+$")
	s3BucketRegexp = regexp.MustCompile("^[a-z0-9][a-z0-9.-]{1,61}[a-z0-9]$")
	s3PrefixRegexp = regexp.MustCompile("^[A-Za-z0-9][A-Za-z0-9_-]*$")
	telegramTokenRegexp = regexp.MustCompile("^[0-9]{5,}:[A-Za-z0-9_-]{20,}$")
	uuidRegexp = regexp.MustCompile("(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$")
	borgmaticVersionRegexp = regexp.MustCompile("^[0-9]+\\.[0-9]+\\.[0-9]+(?:[-+][A-Za-z0-9.-]+)?$")
	hermesDashboardSecretRegexp = regexp.MustCompile("^[a-fA-F0-9]{64}$")
)

var generatedSecretFieldNames = []string{
	"admin_password",
	"postgres_password",
	"nextcloud_redis_pass",
	"secret_vaultwarden_password",
	"hbox_pepper",
	"borgmatic_encryption_passphrase",
	"hermes_dashboard_basic_auth_secret",
}

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

func randomHexSecret(size int) (string, error) {
	if size < 16 || size > 1024 {
		return "", fmt.Errorf("неподдерживаемый размер секрета: %d байт", size)
	}
	raw := make([]byte, size)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("криптографический генератор случайных чисел недоступен: %w", err)
	}
	return hex.EncodeToString(raw), nil
}

func generateSSHKeyPair() (privateKey string, publicKey string, err error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return "", "", fmt.Errorf("не удалось сгенерировать Ed25519 SSH-ключ: %w", err)
	}
	block, err := ssh.MarshalPrivateKey(priv, "ServeHub-2 installer")
	if err != nil {
		return "", "", fmt.Errorf("не удалось сформировать OpenSSH private key: %w", err)
	}
	privateKey = string(pem.EncodeToMemory(block))
	authorizedKey, err := ssh.NewPublicKey(pub)
	if err != nil {
		return "", "", fmt.Errorf("не удалось сериализовать публичный SSH-ключ: %w", err)
	}
	publicKey = strings.TrimSpace(string(ssh.MarshalAuthorizedKey(authorizedKey)))
	return privateKey, publicKey, nil
}

func hasSecretMappingKey(document *yaml.Node, key string) bool {
	if document == nil || document.Kind != yaml.DocumentNode || len(document.Content) != 1 {
		return false
	}
	root := document.Content[0]
	if root.Kind != yaml.MappingNode {
		return false
	}
	for i := 0; i+1 < len(root.Content); i += 2 {
		if root.Content[i].Kind == yaml.ScalarNode && root.Content[i].Tag == "!!str" && root.Content[i].Value == key {
			return true
		}
	}
	return false
}

// GenerateSecretDefaults creates fresh high-entropy values only for empty or
// known template values. It never rotates an existing non-placeholder secret.
// Generated values are returned for preview; only SaveSecrets persists them.
func (a *App) GenerateSecretDefaults() SecretsResult {
	_, document, values, isTemplate, err := loadSecretsSource()
	if err != nil {
		return SecretsResult{Success: false, Message: "Не удалось прочитать исходную конфигурацию: " + err.Error()}
	}
	generated := make(map[string]string)
	for _, key := range generatedSecretFieldNames {
		value := strings.TrimSpace(values[key])
		if !isUnsafeDefaultValue(value) {
			continue
		}
		// An explicitly empty value in an existing secrets.yml may be intentional
		// or tied to an existing deployment; don't rotate it. A missing key is
		// different: it has no existing value and can safely receive a generated
		// default which is shown in the form and persisted on save.
		if !isTemplate && value == "" && hasSecretMappingKey(document, key) {
			continue
		}
		value, err := randomHexSecret(32)
		if err != nil {
			return SecretsResult{Success: false, Message: err.Error()}
		}
		generated[key] = value
	}

	privateNeeds := isUnsafeDefaultValue(values["ssh_private_key"])
	publicNeeds := isUnsafeDefaultValue(values["ssh_public_key"])
	if !isTemplate {
		privateIsExplicitBlank := hasSecretMappingKey(document, "ssh_private_key") && strings.TrimSpace(values["ssh_private_key"]) == ""
		publicIsExplicitBlank := hasSecretMappingKey(document, "ssh_public_key") && strings.TrimSpace(values["ssh_public_key"]) == ""
		if privateIsExplicitBlank || publicIsExplicitBlank {
			privateNeeds = false
			publicNeeds = false
		}
	}
	if privateNeeds && publicNeeds {
		privateKey, publicKey, err := generateSSHKeyPair()
		if err != nil {
			return SecretsResult{Success: false, Message: err.Error()}
		}
		generated["ssh_private_key"] = privateKey
		generated["ssh_public_key"] = publicKey
	}

	return SecretsResult{
		Success: true,
		Message: "Подготовлены криптографически стойкие значения для пустых/шаблонных секретов. Они ещё не записаны в secrets.yml.",
		Values: generated,
	}
}

func validateIPv4(values map[string]string, key, label string) error {
	value := strings.TrimSpace(values[key])
	ip := net.ParseIP(value)
	if ip == nil || ip.To4() == nil {
		return fmt.Errorf("%s: укажите корректный IPv4-адрес", label)
	}
	return nil
}

func validateUsername(values map[string]string, key, label string) error {
	value := strings.TrimSpace(values[key])
	if !deploymentUsernameRegexp.MatchString(value) || value == "root" || isUnsafeDefaultValue(value) {
		return fmt.Errorf("%s: используйте имя Linux-пользователя (буквы a-z, цифры, '_' и '-', до 32 символов; root не подходит для deploy_user)", label)
	}
	return nil
}

func isReservedExampleDomain(value string) bool {
	domain := strings.TrimSuffix(strings.ToLower(strings.TrimSpace(value)), ".")
	if strings.HasPrefix(domain, "example.") {
		return true
	}
	for _, reserved := range []string{"example.com", "example.org", "example.net", "example.edu", ".example.com", ".example.org", ".example.net", ".invalid", ".localhost"} {
		if domain == reserved || strings.HasSuffix(domain, reserved) {
			return true
		}
	}
	return false
}

func validateDomainAndEmail(values map[string]string) error {
	domain := strings.TrimSuffix(strings.TrimSpace(values["server_name"]), ".")
	if len(domain) > 253 || net.ParseIP(domain) != nil || !deploymentDomainRegexp.MatchString(domain) || isReservedExampleDomain(domain) {
		return errors.New("server_name: укажите ваш действующий домен с корректными DNS-метками; example.* и IP-адреса в качестве домена не принимаются")
	}
	emailValue := strings.TrimSpace(values["email"])
	parsed, err := mail.ParseAddress(emailValue)
	if err != nil || parsed.Address != emailValue || !strings.Contains(emailValue, "@") {
		return errors.New("email: укажите корректный адрес электронной почты без отображаемого имени")
	}
	emailDomain := emailValue[strings.LastIndex(emailValue, "@")+1:]
	if isReservedExampleDomain(emailDomain) || isUnsafeDefaultValue(emailValue) {
		return errors.New("email: замените примерный адрес реальным адресом на вашем домене")
	}
	return nil
}

func validateSSHKeyPair(values map[string]string) error {
	privateText := strings.TrimSpace(values["ssh_private_key"])
	publicText := strings.TrimSpace(values["ssh_public_key"])
	if privateText == "" || isUnsafeDefaultValue(privateText) {
		return errors.New("ssh_private_key: укажите приватный SSH-ключ или разрешите установщику создать новую пару")
	}
	if publicText == "" || isUnsafeDefaultValue(publicText) {
		return errors.New("ssh_public_key: укажите публичный SSH-ключ или разрешите установщику создать новую пару")
	}
	signer, err := ssh.ParsePrivateKey([]byte(privateText))
	if err != nil {
		return fmt.Errorf("ssh_private_key: ключ не читается или зашифрован. Нужен корректный незашифрованный deploy-ключ: %w", err)
	}
	publicKey, _, _, _, err := ssh.ParseAuthorizedKey([]byte(publicText))
	if err != nil {
		return fmt.Errorf("ssh_public_key: некорректный формат authorized_keys: %w", err)
	}
	if !bytes.Equal(signer.PublicKey().Marshal(), publicKey.Marshal()) {
		return errors.New("ssh_private_key и ssh_public_key относятся к разным ключевым парам. Исправьте пару до деплоя")
	}
	return nil
}

func hasLowEntropySecret(value string, minLength int) bool {
	value = strings.TrimSpace(value)
	if len(value) < minLength {
		return true
	}
	lower := strings.ToLower(value)
	switch lower {
	case "passwordpasswordpasswordpassword",
		"changemechangemechangemechangeme",
		"12345678901234567890123456789012",
		"qwertyuiopasdfghjklzxcvbnmqwerty",
		"adminadminadminadminadminadminadmin":
		return true
	}

	// Reject a string made by repeating a tiny pattern (e.g. "abcdabcd...")
	for patternLength := 1; patternLength <= 4 && patternLength*3 <= len(value); patternLength++ {
		if len(value)%patternLength != 0 {
			continue
		}
		pattern := value[:patternLength]
		if strings.Repeat(pattern, len(value)/patternLength) == value {
			return true
		}
	}

	unique := make(map[rune]struct{})
	for _, char := range value {
		unique[char] = struct{}{}
	}
	minUnique := 6
	if len(value) >= 32 {
		minUnique = 8
	}
	return len(unique) < minUnique
}

func validateStrongSecret(values map[string]string, key, label string, minLength int) error {
	value := strings.TrimSpace(values[key])
	if value == "" || isUnsafeDefaultValue(value) {
		return fmt.Errorf("%s: значение пустое или осталось шаблонным. Сгенерируйте секрет либо задайте собственный", label)
	}
	if len(value) < minLength {
		return fmt.Errorf("%s: требуется не менее %d байт", label, minLength)
	}
	if hasLowEntropySecret(value, minLength) {
		return fmt.Errorf("%s: секрет слишком простой или повторяющийся. Используйте случайное значение с достаточным разнообразием символов", label)
	}
	if strings.ContainsAny(value, "\r\n\x00") {
		return fmt.Errorf("%s: переносы строк и нулевой байт не допускаются", label)
	}
	return nil
}

func usableCredential(values map[string]string, key string) bool {
	value := strings.TrimSpace(values[key])
	return value != "" && !isUnsafeDefaultValue(value)
}

func validateBootstrapCredentials(values map[string]string, prefix, label string) error {
	rootKey := prefix + "_root_password"
	sshKey := prefix + "_ssh_password"
	sudoKey := prefix + "_sudo_password"
	rootAvailable := usableCredential(values, rootKey)
	sshAvailable := usableCredential(values, sshKey)
	sudoAvailable := usableCredential(values, sudoKey)
	for _, key := range []string{rootKey, sshKey, sudoKey} {
		value := strings.TrimSpace(values[key])
		if value != "" && isUnsafeDefaultValue(value) {
			return fmt.Errorf("%s: %s содержит шаблонное значение; оставьте поле пустым или введите настоящий пароль", label, key)
		}
		if value != "" {
			if err := validateStrongSecret(values, key, label+" "+key, 12); err != nil {
				return err
			}
		}
	}
	if !rootAvailable && !(sshAvailable && sudoAvailable) {
		return fmt.Errorf("%s: для полного bootstrap задайте пароль root или обе пары %s и %s для SSH/sudo", label, sshKey, sudoKey)
	}
	return nil
}

func validateS3Configuration(values map[string]string, usesLocal, usesVPS bool) error {
	required := []struct{ key, label string }{
		{"borgmatic_encryption_passphrase", "borgmatic_encryption_passphrase"},
		{"offsite_s3_endpoint", "offsite_s3_endpoint"},
		{"offsite_s3_region", "offsite_s3_region"},
		{"offsite_s3_bucket", "offsite_s3_bucket"},
		{"offsite_s3_access_key_id", "offsite_s3_access_key_id"},
		{"offsite_s3_secret_access_key", "offsite_s3_secret_access_key"},
	}
	for _, field := range required {
		value := strings.TrimSpace(values[field.key])
		if value == "" || isUnsafeDefaultValue(value) {
			return fmt.Errorf("резервное копирование включено для всех сценариев: заполните %s", field.label)
		}
	}
	if err := validateStrongSecret(values, "offsite_s3_access_key_id", "offsite_s3_access_key_id", 16); err != nil {
		return err
	}
	if err := validateStrongSecret(values, "offsite_s3_secret_access_key", "offsite_s3_secret_access_key", 32); err != nil {
		return err
	}
	if err := validateStrongSecret(values, "borgmatic_encryption_passphrase", "borgmatic_encryption_passphrase", 32); err != nil {
		return err
	}
	endpoint, err := url.Parse(strings.TrimSpace(values["offsite_s3_endpoint"]))
	if err != nil || endpoint.Scheme != "https" || endpoint.Host == "" || endpoint.User != nil ||
		(endpoint.Path != "" && endpoint.Path != "/") || endpoint.RawQuery != "" || endpoint.Fragment != "" {
		return errors.New("offsite_s3_endpoint: требуется HTTPS endpoint без пути, логина, query-параметров и fragment")
	}
	bucket := strings.TrimSpace(values["offsite_s3_bucket"])
	if !s3BucketRegexp.MatchString(bucket) || strings.Contains(bucket, "..") {
		return errors.New("offsite_s3_bucket: некорректное S3-имя бакета")
	}
	workload := strings.TrimSpace(values["offsite_s3_workload_prefix"])
	edge := strings.TrimSpace(values["offsite_s3_edge_prefix"])
	if usesLocal && (workload == "" || !s3PrefixRegexp.MatchString(workload)) {
		return errors.New("offsite_s3_workload_prefix: требуется корректный префикс для локального Workload")
	}
	if usesVPS && (edge == "" || !s3PrefixRegexp.MatchString(edge)) {
		return errors.New("offsite_s3_edge_prefix: требуется корректный префикс для VPS/Edge")
	}
	if usesLocal && usesVPS && workload == edge {
		return errors.New("offsite_s3_workload_prefix/offsite_s3_edge_prefix: для развёртывания обоих серверов задайте разные префиксы")
	}
	return nil
}

func validateHermesConfiguration(values map[string]string) error {
	baseURL := strings.TrimSpace(values["hermes_api_base_url"])
	parsed, err := url.Parse(baseURL)
	if err != nil || (parsed.Scheme != "https" && parsed.Scheme != "http") ||
		parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return errors.New("hermes_api_base_url: укажите корректный HTTP(S) API URL без логина, пароля, query и fragment")
	}
	apiKey := strings.TrimSpace(values["hermes_api_key"])
	if len(apiKey) < 16 || isUnsafeDefaultValue(apiKey) || strings.ContainsAny(apiKey, "\r\n\x00") {
		return errors.New("hermes_api_key: нужен настоящий API-ключ провайдера Hermes длиной не менее 16 символов; его нельзя сгенерировать локально")
	}
	model := strings.TrimSpace(values["hermes_model"])
	if model == "" || isUnsafeDefaultValue(model) || strings.ContainsAny(model, "\r\n\x00") {
		return errors.New("hermes_model: укажите поддерживаемое провайдером имя модели")
	}
	if err := validateStrongSecret(values, "hermes_dashboard_basic_auth_secret", "hermes_dashboard_basic_auth_secret", 64); err != nil {
		return err
	}
	if !hermesDashboardSecretRegexp.MatchString(strings.TrimSpace(values["hermes_dashboard_basic_auth_secret"])) {
		return errors.New("hermes_dashboard_basic_auth_secret: ожидается 64-символьный hex-секрет")
	}
	return nil
}

func validateTelegramConfiguration(values map[string]string) error {
	token := strings.TrimSpace(values["telegram_token"])
	if !telegramTokenRegexp.MatchString(token) {
		return errors.New("telegram_token: задайте настоящий токен Telegram-бота в формате <bot_id>:<token>")
	}
	chatID := strings.TrimSpace(values["telegram_chat_id"])
	parsed, err := strconv.ParseInt(chatID, 10, 64)
	if err != nil || parsed == 0 {
		return errors.New("telegram_chat_id: укажите числовой Telegram Chat ID, не равный нулю")
	}
	return nil
}

func validateWebnamesKey(values map[string]string) error {
	value := strings.TrimSpace(values["webnames_apikey"])
	if value == "" || isUnsafeDefaultValue(value) || len(value) < 16 {
		return errors.New("webnames_apikey: для выпуска сертификатов на локальном сервере укажите настоящий API-ключ Webnames.ru")
	}
	return nil
}

func validateDeploymentValues(values map[string]string, option int) error {
	if option < 1 || option > 6 {
		return fmt.Errorf("неизвестный сценарий деплоя: %d", option)
	}
	usesVPS := option == 1 || option == 2 || option == 5 || option == 6
	usesLocal := option >= 1 && option <= 4
	doesBootstrap := option == 1 || option == 3 || option == 5

	if err := validateUsername(values, "admin_user", "admin_user"); err != nil {
		return err
	}
	if err := validateSSHKeyPair(values); err != nil {
		return err
	}
	if err := validateStrongSecret(values, "admin_password", "admin_password", 32); err != nil {
		return err
	}
	if err := validateS3Configuration(values, usesLocal, usesVPS); err != nil {
		return err
	}

	if usesVPS {
		if err := validateIPv4(values, "vps_public_ip", "vps_public_ip (VPS)"); err != nil {
			return err
		}
		if err := validateUsername(values, "vps_user", "vps_user"); err != nil {
			return err
		}
		if doesBootstrap {
			if err := validateBootstrapCredentials(values, "vps", "VPS"); err != nil {
				return err
			}
		}
		if err := validateTelegramConfiguration(values); err != nil {
			return err
		}
		if err := validateHermesConfiguration(values); err != nil {
			return err
		}
	}

	if usesLocal {
		if err := validateIPv4(values, "local_private_ip", "local_private_ip (домашний сервер)"); err != nil {
			return err
		}
		if err := validateUsername(values, "local_user", "local_user"); err != nil {
			return err
		}
		if doesBootstrap {
			if err := validateBootstrapCredentials(values, "local", "домашний сервер"); err != nil {
				return err
			}
		}
		if err := validateDomainAndEmail(values); err != nil {
			return err
		}
		if err := validateWebnamesKey(values); err != nil {
			return err
		}
		for _, field := range []struct{ key, label string }{
			{"postgres_password", "postgres_password"},
			{"nextcloud_redis_pass", "nextcloud_redis_pass"},
			{"secret_vaultwarden_password", "secret_vaultwarden_password"},
			{"hbox_pepper", "hbox_pepper"},
		} {
			if err := validateStrongSecret(values, field.key, field.label, 32); err != nil {
				return err
			}
		}
		if !serviceUsernameRegexp.MatchString(strings.TrimSpace(values["postgres_user"])) ||
			isUnsafeDefaultValue(values["postgres_user"]) {
			return errors.New("postgres_user: укажите корректное имя пользователя PostgreSQL")
		}
	}

	if uuid := strings.TrimSpace(values["backup_disk_uuid"]); uuid != "" && !uuidRegexp.MatchString(uuid) {
		return errors.New("backup_disk_uuid: укажите корректный UUID диска или оставьте поле пустым")
	}
	if version := strings.TrimSpace(values["borgmatic_version"]); version != "" && !borgmaticVersionRegexp.MatchString(version) {
		return errors.New("borgmatic_version: ожидается версия, например 2.1.10, либо пустое значение для версии из playbook")
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
	return CheckResult{Success: true, Message: "Секреты и параметры выбранного сценария прошли проверку."}
}

func validateSavedSecretsForDeployment(option int) error {
	return validateDeploymentSource(nil, option)
}
