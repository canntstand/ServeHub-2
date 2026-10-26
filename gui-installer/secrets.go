package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

var editableSecretFields = map[string]struct{}{
	"vps_public_ip": {},
	"vps_user": {},
	"vps_root_password": {},
	"vps_ssh_password": {},
	"vps_sudo_password": {},
	"local_private_ip": {},
	"local_user": {},
	"local_root_password": {},
	"local_ssh_password": {},
	"local_sudo_password": {},
	"server_name": {},
	"admin_user": {},
	"admin_password": {},
	"email": {},
	"telegram_token": {},
	"telegram_chat_id": {},
	"webnames_apikey": {},
	"postgres_user": {},
	"postgres_password": {},
	"nextcloud_redis_pass": {},
	"secret_vaultwarden_password": {},
	"hbox_pepper": {},
	"ssh_public_key": {},
	"ssh_private_key": {},
	"borgmatic_encryption_passphrase": {},
	"backup_disk_uuid": {},
	"offsite_s3_bucket": {},
	"offsite_s3_access_key_id": {},
	"offsite_s3_secret_access_key": {},
	"hermes_api_base_url": {},
	"hermes_api_key": {},
	"hermes_model": {},
	"hermes_dashboard_basic_auth_secret": {},
}

var sensitiveSecretFields = map[string]struct{}{
	"vps_root_password":                  {},
	"vps_ssh_password":                   {},
	"vps_sudo_password":                  {},
	"local_root_password":                {},
	"local_ssh_password":                 {},
	"local_sudo_password":                {},
	"admin_password":                     {},
	"telegram_token":                     {},
	"webnames_apikey":                    {},
	"postgres_password":                  {},
	"nextcloud_redis_pass":               {},
	"secret_vaultwarden_password":        {},
	"hbox_pepper":                        {},
	"adguard_password_salt":              {},
	"ssh_private_key":                    {},
	"borgmatic_encryption_passphrase":    {},
	"offsite_s3_access_key_id":           {},
	"offsite_s3_secret_access_key":        {},
	"hermes_api_key":                     {},
	"hermes_dashboard_basic_auth_secret": {},
}

func isSensitiveSecretField(key string) bool {
	if _, ok := sensitiveSecretFields[key]; ok {
		return true
	}
	key = strings.ToLower(key)
	for _, marker := range []string{
		"password", "passphrase", "token", "secret", "api_key", "apikey",
		"access_key", "private_key", "pepper", "salt", "credential",
	} {
		if strings.Contains(key, marker) {
			return true
		}
	}
	return false
}

// redactSensitiveText masks complete secret values and individual lines of
// multiline secrets (notably PEM/OpenSSH private keys) before diagnostics or
// process output are forwarded to the UI.
func redactSensitiveText(text string, values map[string]string) string {
	candidates := make([]string, 0, len(values)*2)
	seen := make(map[string]struct{}, len(values)*2)
	addCandidate := func(candidate string) {
		candidate = strings.TrimSpace(candidate)
		if len(candidate) < 4 {
			return
		}
		if _, exists := seen[candidate]; exists {
			return
		}
		seen[candidate] = struct{}{}
		candidates = append(candidates, candidate)
	}

	for key, value := range values {
		if !isSensitiveSecretField(key) {
			continue
		}
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		addCandidate(value)
		// Some tools print multiline values as one escaped string.
		addCandidate(strings.ReplaceAll(value, "\n", "\\n"))
		for _, line := range strings.FieldsFunc(value, func(r rune) bool {
			return r == '\n' || r == '\r'
		}) {
			addCandidate(line)
		}
	}

	sort.Slice(candidates, func(i, j int) bool {
		return len(candidates[i]) > len(candidates[j])
	})
	for _, candidate := range candidates {
		text = strings.ReplaceAll(text, candidate, "[REDACTED]")
	}
	return text
}

func secretsPaths() (string, string, string, error) {
	projectDir, err := configuredProjectInstallDirectory()
	if err != nil {
		return "", "", "", err
	}
	dir := filepath.Join(projectDir, "ansible", "vars")
	return dir,
		filepath.Join(dir, "secrets.yml"),
		filepath.Join(dir, "secrets.yml.example"),
		nil
}

func parseSecretsYAML(content []byte) (*yaml.Node, error) {
	if len(bytes.TrimSpace(content)) == 0 {
		return nil, errors.New("YAML-файл пуст")
	}

	decoder := yaml.NewDecoder(bytes.NewReader(content))
	var document yaml.Node
	if err := decoder.Decode(&document); err != nil {
		return nil, fmt.Errorf("ошибка синтаксиса YAML: %w", err)
	}
	var extra yaml.Node
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return nil, errors.New("несколько YAML-документов в secrets.yml не поддерживаются")
		}
		return nil, fmt.Errorf("ошибка чтения YAML: %w", err)
	}
	if document.Kind != yaml.DocumentNode || len(document.Content) != 1 {
		return nil, errors.New("ожидался один YAML-документ")
	}
	root := document.Content[0]
	if root.Kind != yaml.MappingNode || root.Tag != "!!map" {
		return nil, errors.New("корень secrets.yml должен быть YAML-словарём")
	}
	for i := 0; i+1 < len(root.Content); i += 2 {
		key := root.Content[i]
		if key.Kind != yaml.ScalarNode || key.Tag != "!!str" {
			return nil, fmt.Errorf("$: ключи верхнего уровня должны быть строками (пара %d)", i/2+1)
		}
	}
	if err := validateSecretYAMLNode(root, "$"); err != nil {
		return nil, err
	}
	return &document, nil
}

func validateSecretYAMLNode(node *yaml.Node, location string) error {
	if node == nil {
		return fmt.Errorf("%s: отсутствует YAML-узел", location)
	}
	if node.Kind == yaml.MappingNode {
		if len(node.Content)%2 != 0 {
			return fmt.Errorf("%s: повреждённый YAML-словарь", location)
		}
		seen := make(map[string]struct{}, len(node.Content)/2)
		for i := 0; i < len(node.Content); i += 2 {
			key := node.Content[i]
			// YAML permits non-string mapping keys and merge keys in nested
			// values. The secrets root is validated separately as string-keyed.
			if key.Kind != yaml.ScalarNode || key.Tag != "!!str" {
				continue
			}
			identity := key.Tag + "\x00" + key.Value
			if _, exists := seen[identity]; exists {
				return fmt.Errorf("%s: повторяющийся YAML-ключ %q", location, key.Value)
			}
			seen[identity] = struct{}{}
		}
	}
	for i, child := range node.Content {
		if err := validateSecretYAMLNode(child, fmt.Sprintf("%s[%d]", location, i)); err != nil {
			return err
		}
	}
	return nil
}

func secretScalarValues(document *yaml.Node) map[string]string {
	values := make(map[string]string)
	if document == nil || document.Kind != yaml.DocumentNode || len(document.Content) != 1 {
		return values
	}
	root := document.Content[0]
	if root.Kind != yaml.MappingNode {
		return values
	}
	for i := 0; i+1 < len(root.Content); i += 2 {
		key, value := root.Content[i], root.Content[i+1]
		if key.Kind != yaml.ScalarNode {
			continue
		}
		if value.Kind == yaml.AliasNode && value.Alias != nil {
			value = value.Alias
		}
		if value.Kind != yaml.ScalarNode {
			continue
		}
		if value.Tag == "!!null" {
			values[key.Value] = ""
		} else {
			values[key.Value] = value.Value
		}
	}
	return values
}

func loadSecretsSource() ([]byte, *yaml.Node, map[string]string, bool, error) {
	_, targetPath, templatePath, pathErr := secretsPaths()
	if pathErr != nil {
		return nil, nil, nil, false, fmt.Errorf("не удалось определить каталог secrets.yml: %w", pathErr)
	}
	targetInfo, err := os.Lstat(targetPath)
	if err == nil {
		if targetInfo.Mode()&os.ModeSymlink != 0 || !targetInfo.Mode().IsRegular() {
			return nil, nil, nil, false, errors.New("secrets.yml должен быть обычным файлом, а не ссылкой или специальным файлом")
		}
		content, readErr := os.ReadFile(targetPath)
		if readErr != nil {
			return nil, nil, nil, false, fmt.Errorf("не удалось прочитать secrets.yml: %w", readErr)
		}
		document, parseErr := parseSecretsYAML(content)
		if parseErr != nil {
			return nil, nil, nil, false, fmt.Errorf("secrets.yml не прошёл проверку: %w", parseErr)
		}
		return content, document, secretScalarValues(document), false, nil
	}
	if !os.IsNotExist(err) {
		return nil, nil, nil, false, fmt.Errorf("не удалось проверить secrets.yml: %w", err)
	}

	templateInfo, err := os.Lstat(templatePath)
	if err != nil {
		return nil, nil, nil, true, fmt.Errorf("secrets.yml ещё не создан, а шаблон secrets.yml.example недоступен: %w", err)
	}
	if templateInfo.Mode()&os.ModeSymlink != 0 || !templateInfo.Mode().IsRegular() {
		return nil, nil, nil, true, errors.New("secrets.yml.example должен быть обычным файлом")
	}
	content, err := os.ReadFile(templatePath)
	if err != nil {
		return nil, nil, nil, true, fmt.Errorf("не удалось прочитать шаблон secrets.yml.example: %w", err)
	}
	document, err := parseSecretsYAML(content)
	if err != nil {
		return nil, nil, nil, true, fmt.Errorf("шаблон secrets.yml.example не прошёл проверку: %w", err)
	}
	return content, document, secretScalarValues(document), true, nil
}

const bootstrapAuthDeletePrefix = "__delete_auth__"

func isBootstrapAuthSecretField(key string) bool {
	switch key {
	case "vps_root_password", "vps_ssh_password", "vps_sudo_password",
		"local_root_password", "local_ssh_password", "local_sudo_password":
		return true
	default:
		return false
	}
}

func removeSecretMappingKey(root *yaml.Node, key string) {
	if root == nil || root.Kind != yaml.MappingNode {
		return
	}
	for i := 0; i+1 < len(root.Content); i += 2 {
		if root.Content[i].Kind == yaml.ScalarNode && root.Content[i].Value == key {
			root.Content = append(root.Content[:i], root.Content[i+2:]...)
			return
		}
	}
}

func mergeSecretUpdates(document *yaml.Node, updates map[string]string) error {
	if document == nil || document.Kind != yaml.DocumentNode || len(document.Content) != 1 ||
		document.Content[0].Kind != yaml.MappingNode {
		return errors.New("невозможно обновить: корень secrets.yml не является словарём")
	}
	root := document.Content[0]

	keys := make([]string, 0, len(updates))
	for key, value := range updates {
		if strings.HasPrefix(key, bootstrapAuthDeletePrefix) {
			targetKey := strings.TrimPrefix(key, bootstrapAuthDeletePrefix)
			if !isBootstrapAuthSecretField(targetKey) || value != "true" {
				return fmt.Errorf("операция удаления поля %q не поддерживается", targetKey)
			}
			if _, conflicts := updates[targetKey]; conflicts {
				return fmt.Errorf("поле %q одновременно изменяется и удаляется", targetKey)
			}
			keys = append(keys, key)
			continue
		}
		if _, allowed := editableSecretFields[key]; !allowed {
			return fmt.Errorf("поле %q не поддерживается формой; неизвестные поля конфигурации сохраняются без изменений", key)
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)

	for _, key := range keys {
		if strings.HasPrefix(key, bootstrapAuthDeletePrefix) {
			removeSecretMappingKey(root, strings.TrimPrefix(key, bootstrapAuthDeletePrefix))
			continue
		}

		value := updates[key]
		pairIndex := -1
		for i := 0; i+1 < len(root.Content); i += 2 {
			if root.Content[i].Kind == yaml.ScalarNode && root.Content[i].Value == key {
				pairIndex = i
				break
			}
		}
		// A blank form field is not proof that an existing credential should
		// be removed. Preserve populated secrets unless a replacement is given;
		// inactive bootstrap credentials use the explicit deletion operation above.
		if pairIndex >= 0 && isSensitiveSecretField(key) && strings.TrimSpace(value) == "" {
			oldValue := root.Content[pairIndex+1]
			if oldValue.Kind != yaml.ScalarNode ||
				(strings.TrimSpace(oldValue.Value) != "" && !isUnsafeDefaultValue(oldValue.Value)) {
				continue
			}
		}

		newValue := &yaml.Node{
			Kind:  yaml.ScalarNode,
			Tag:   "!!str",
			Value: value,
			Style: yaml.DoubleQuotedStyle,
		}
		if strings.Contains(value, "\n") {
			newValue.Style = yaml.LiteralStyle
		}

		if pairIndex >= 0 {
			oldValue := root.Content[pairIndex+1]
			newValue.HeadComment = oldValue.HeadComment
			newValue.LineComment = oldValue.LineComment
			newValue.FootComment = oldValue.FootComment
			root.Content[pairIndex+1] = newValue
		} else {
			root.Content = append(root.Content,
				&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key},
				newValue,
			)
		}
	}

	return validateSecretYAMLNode(root, "$")
}
func marshalSecretsYAML(document *yaml.Node) ([]byte, error) {
	if _, err := parseSecretsYAMLFromNode(document); err != nil {
		return nil, err
	}
	content, err := yaml.Marshal(document)
	if err != nil {
		return nil, fmt.Errorf("не удалось сериализовать YAML: %w", err)
	}
	if _, err := parseSecretsYAML(content); err != nil {
		return nil, fmt.Errorf("сформированный YAML не прошёл повторную проверку: %w", err)
	}
	return content, nil
}

func parseSecretsYAMLFromNode(document *yaml.Node) (*yaml.Node, error) {
	if document == nil || document.Kind != yaml.DocumentNode || len(document.Content) != 1 {
		return nil, errors.New("ожидался один YAML-документ")
	}
	if document.Content[0].Kind != yaml.MappingNode {
		return nil, errors.New("корень secrets.yml должен быть YAML-словарём")
	}
	if err := validateSecretYAMLNode(document.Content[0], "$"); err != nil {
		return nil, err
	}
	return document, nil
}

func secretYAMLEquivalent(left, right *yaml.Node) bool {
	if left == nil || right == nil {
		return left == right
	}
	if left.Kind != right.Kind || left.Tag != right.Tag || left.Value != right.Value ||
		left.Anchor != right.Anchor || len(left.Content) != len(right.Content) {
		return false
	}
	if left.Kind == yaml.AliasNode {
		leftAnchor, rightAnchor := "", ""
		if left.Alias != nil {
			leftAnchor = left.Alias.Anchor
		}
		if right.Alias != nil {
			rightAnchor = right.Alias.Anchor
		}
		if leftAnchor != rightAnchor {
			return false
		}
	}
	for i := range left.Content {
		if !secretYAMLEquivalent(left.Content[i], right.Content[i]) {
			return false
		}
	}
	return true
}

func (a *App) CheckSecrets() CheckResult {
	_, targetPath, _, pathErr := secretsPaths()
	if pathErr != nil {
		return CheckResult{Success: false, Message: "Не удалось определить каталог secrets.yml: " + pathErr.Error()}
	}
	info, err := os.Lstat(targetPath)
	if os.IsNotExist(err) {
		return CheckResult{Success: false, Exists: false, Message: "Файл secrets.yml не найден; будет использован шаблон secrets.yml.example."}
	}
	if err != nil {
		return CheckResult{Success: false, Exists: true, Message: "Не удалось проверить secrets.yml: " + err.Error()}
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return CheckResult{Success: false, Exists: true, Message: "secrets.yml должен быть обычным файлом, а не символической ссылкой."}
	}
	content, err := os.ReadFile(targetPath)
	if err != nil {
		return CheckResult{Success: false, Exists: true, Message: "Не удалось прочитать secrets.yml: " + err.Error()}
	}
	if _, err := parseSecretsYAML(content); err != nil {
		return CheckResult{Success: false, Exists: true, Message: "Текущий secrets.yml содержит ошибку YAML: " + err.Error()}
	}
	return CheckResult{Success: true, Exists: true, Message: "Текущий secrets.yml прочитан и проверен YAML-парсером."}
}

func (a *App) LoadSecrets() SecretsResult {
	content, _, values, isTemplate, err := loadSecretsSource()
	if err != nil {
		return SecretsResult{Success: false, Message: err.Error()}
	}
	message := "Текущий secrets.yml загружен и проверен."
	if isTemplate {
		message = "Загружен шаблон secrets.yml.example; неизвестные и дополнительные поля шаблона сохраняются при редактировании."
	}
	return SecretsResult{
		Success: true,
		Message: message,
		Content: string(content),
		Values: values,
		IsTemplate: isTemplate,
	}
}

// PreviewSecrets merges changed form fields into the parsed current configuration.
// Keys absent from the form, including custom settings and nested YAML, are retained.
func (a *App) PreviewSecrets(updates map[string]string) SecretsResult {
	_, document, _, _, err := loadSecretsSource()
	if err != nil {
		return SecretsResult{Success: false, Message: err.Error()}
	}
	if err := mergeSecretUpdates(document, updates); err != nil {
		return SecretsResult{Success: false, Message: "Не удалось обновить конфигурацию: " + err.Error()}
	}
	content, err := marshalSecretsYAML(document)
	if err != nil {
		return SecretsResult{Success: false, Message: "Не удалось проверить итоговый YAML: " + err.Error()}
	}
	verified, err := parseSecretsYAML(content)
	if err != nil || !secretYAMLEquivalent(document, verified) {
		if err == nil {
			err = errors.New("структура YAML изменилась после сериализации")
		}
		return SecretsResult{Success: false, Message: "Итоговый YAML не прошёл проверку: " + err.Error()}
	}
	return SecretsResult{
		Success: true,
		Message: "Итоговый secrets.yml разобран, объединён с исходной конфигурацией и проверен.",
		Content: string(content),
		Values: secretScalarValues(verified),
	}
}


func (a *App) SaveSecrets(updates map[string]string, expectedYAML string) CheckResult {
	dir, targetPath, _, pathErr := secretsPaths()
	if pathErr != nil {
		return CheckResult{Success: false, Message: "Файл не сохранён: не удалось определить каталог secrets.yml: " + pathErr.Error()}
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return CheckResult{Success: false, Message: "Не удалось создать каталог конфигурации: " + err.Error()}
	}

	_, document, _, _, err := loadSecretsSource()
	if err != nil {
		return CheckResult{Success: false, Message: "Файл не сохранён: не удалось прочитать исходную конфигурацию: " + err.Error()}
	}
	if err := mergeSecretUpdates(document, updates); err != nil {
		return CheckResult{Success: false, Message: "Файл не сохранён: не удалось применить изменения: " + err.Error()}
	}
	normalized, err := marshalSecretsYAML(document)
	if err != nil {
		return CheckResult{Success: false, Message: "Файл не сохранён: итоговый YAML не прошёл проверку: " + err.Error()}
	}

	previewDocument, err := parseSecretsYAML([]byte(expectedYAML))
	if err != nil {
		return CheckResult{Success: false, Message: "Файл не сохранён: YAML предпросмотра не прошёл повторную проверку: " + err.Error()}
	}
	if !secretYAMLEquivalent(document, previewDocument) {
		return CheckResult{Success: false, Message: "Файл не сохранён: исходная конфигурация изменилась после предпросмотра. Вернитесь к форме и заново проверьте итоговый YAML."}
	}

	if err := writeSecretsAtomically(targetPath, normalized); err != nil {
		return CheckResult{Success: false, Message: "Не удалось безопасно записать secrets.yml: " + err.Error()}
	}

	readBack, err := os.ReadFile(targetPath)
	if err != nil {
		return CheckResult{Success: false, Message: "Файл записан, но повторное чтение secrets.yml не удалось: " + err.Error()}
	}
	readDocument, err := parseSecretsYAML(readBack)
	if err != nil {
		return CheckResult{Success: false, Message: "Файл записан, но повторная проверка YAML не пройдена: " + err.Error()}
	}
	if !secretYAMLEquivalent(document, readDocument) {
		return CheckResult{Success: false, Message: "Файл записан, но содержимое после чтения отличается от проверенной конфигурации."}
	}
	info, err := os.Stat(targetPath)
	if err != nil {
		return CheckResult{Success: false, Message: "Не удалось проверить права сохранённого secrets.yml: " + err.Error()}
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0600 {
		return CheckResult{Success: false, Message: fmt.Sprintf("Неверные права secrets.yml: %04o вместо 0600", info.Mode().Perm())}
	}
	return CheckResult{
		Success: true,
		Exists:  true,
		Message: "secrets.yml сохранён атомарно, повторно прочитан, проверен YAML-парсером и имеет права 0600 на Linux.",
	}
}

func writeSecretsAtomically(targetPath string, content []byte) error {
	if _, err := parseSecretsYAML(content); err != nil {
		return fmt.Errorf("отказ от записи непроверенного YAML: %w", err)
	}
	targetInfo, err := os.Lstat(targetPath)
	targetExists := err == nil
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if targetExists && (targetInfo.Mode()&os.ModeSymlink != 0 || !targetInfo.Mode().IsRegular()) {
		return errors.New("существующий путь secrets.yml не является обычным файлом")
	}

	dir := filepath.Dir(targetPath)
	tempFile, err := os.CreateTemp(dir, ".secrets.yml.tmp-*")
	if err != nil {
		return err
	}
	tempPath := tempFile.Name()
	defer os.Remove(tempPath)

	if err := tempFile.Chmod(0600); err != nil {
		tempFile.Close()
		return err
	}
	if _, err := tempFile.Write(content); err != nil {
		tempFile.Close()
		return err
	}
	if err := tempFile.Sync(); err != nil {
		tempFile.Close()
		return err
	}
	if err := tempFile.Close(); err != nil {
		return err
	}

	if runtime.GOOS != "windows" || !targetExists {
		if err := os.Rename(tempPath, targetPath); err != nil {
			return err
		}
	} else {
		backupFile, err := os.CreateTemp(dir, ".secrets.yml.backup-*")
		if err != nil {
			return err
		}
		backupPath := backupFile.Name()
		if err := backupFile.Close(); err != nil {
			os.Remove(backupPath)
			return err
		}
		if err := os.Remove(backupPath); err != nil {
			return err
		}
		if err := os.Rename(targetPath, backupPath); err != nil {
			return err
		}
		if err := os.Rename(tempPath, targetPath); err != nil {
			restoreErr := os.Rename(backupPath, targetPath)
			if restoreErr != nil {
				return fmt.Errorf("не удалось заменить secrets.yml (%v) и восстановить предыдущий файл (%v); резервная копия: %s", err, restoreErr, backupPath)
			}
			return err
		}
		if err := os.Remove(backupPath); err != nil {
			return fmt.Errorf("secrets.yml обновлён, но временную резервную копию не удалось удалить: %w", err)
		}
	}

	if err := os.Chmod(targetPath, 0600); err != nil {
		return fmt.Errorf("файл записан, но не удалось назначить права 0600: %w", err)
	}
	return nil
}

