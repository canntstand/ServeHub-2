package main

import (
	"archive/zip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const (
	projectInstallDirName = "ServeHub-2-dev"
	projectRepositoryAPI  = "https://api.github.com/repos/canntstand/ServeHub-2"
	maxProjectArchiveSize = int64(256 << 20)
	maxExpandedProjectSize = uint64(512 << 20)
	maxProjectArchiveFiles = 50000
)

var (
	fullCommitSHARegexp = regexp.MustCompile("^[0-9a-fA-F]{40}$")
	inputCommitSHARegexp = regexp.MustCompile("^[0-9a-fA-F]{7,40}$")
	gitModuleRegexp = regexp.MustCompile("^([ \t]*)(?:ansible\\.builtin\\.)?git:[ \t]*(?:#.*)?$")
	yamlRepoRegexp = regexp.MustCompile("^[ \t]*repo:[ \t]*(.+?)[ \t]*$")
	yamlDestRegexp = regexp.MustCompile("^[ \t]*dest:[ \t]*(.+?)[ \t]*$")
	yamlVersionRegexp = regexp.MustCompile("^([ \t]*)version:[ \t]*(.*?)[ \t]*$")
)

type ProjectVersionChoice struct {
	Mode        string `json:"mode"`
	Ref         string `json:"ref"`
	CommitSHA   string `json:"commitSHA"`
	Label       string `json:"label"`
	PublishedAt string `json:"publishedAt,omitempty"`
}

type ProjectVersionMetadata struct {
	SchemaVersion      int    `json:"schema_version"`
	SourceMode         string `json:"source_mode"`
	SourceRef          string `json:"source_ref"`
	ResolvedRef        string `json:"resolved_ref"`
	CommitSHA          string `json:"commit_sha"`
	DisplayName        string `json:"display_name"`
	InstalledAt        string `json:"installed_at"`
	CompatibilityPatch string `json:"compatibility_patch,omitempty"`
}

type ProjectInstallState struct {
	Exists       bool                   `json:"exists"`
	KnownVersion bool                   `json:"knownVersion"`
	Metadata     ProjectVersionMetadata `json:"metadata"`
	Message      string                 `json:"message"`
}

type ProjectVersionsResult struct {
	Success       bool                 `json:"success"`
	Message       string               `json:"message"`
	LatestRelease ProjectVersionChoice `json:"latestRelease"`
	Tags          []ProjectVersionChoice `json:"tags"`
	Dev           ProjectVersionChoice `json:"dev"`
	Installed     ProjectInstallState  `json:"installed"`
}

type ProjectVersionResolution struct {
	Success     bool   `json:"success"`
	Message     string `json:"message"`
	Mode        string `json:"mode"`
	Ref         string `json:"ref"`
	CommitSHA   string `json:"commitSHA"`
	Label       string `json:"label"`
}

type githubRelease struct {
	TagName     string `json:"tag_name"`
	Name        string `json:"name"`
	Draft       bool   `json:"draft"`
	Prerelease  bool   `json:"prerelease"`
	PublishedAt string `json:"published_at"`
}

type githubTag struct {
	Name   string `json:"name"`
	Commit struct {
		SHA string `json:"sha"`
	} `json:"commit"`
}

type githubBranch struct {
	Commit struct {
		SHA string `json:"sha"`
	} `json:"commit"`
}

type githubCommit struct {
	SHA string `json:"sha"`
}

func isFullCommitSHA(value string) bool {
	return fullCommitSHARegexp.MatchString(strings.TrimSpace(value))
}

func isInputCommitSHA(value string) bool {
	return inputCommitSHARegexp.MatchString(strings.TrimSpace(value))
}

func githubJSON(endpoint string, target any) error {
	request, err := http.NewRequest(http.MethodGet, projectRepositoryAPI+endpoint, nil)
	if err != nil {
		return fmt.Errorf("не удалось подготовить запрос к GitHub: %w", err)
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("User-Agent", "ServeHub-2-Installer")
	request.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	client := &http.Client{Timeout: 25 * time.Second}
	response, err := client.Do(request)
	if err != nil {
		return fmt.Errorf("ошибка запроса к GitHub: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
		detail := strings.TrimSpace(string(body))
		if detail == "" {
			detail = response.Status
		}
		return fmt.Errorf("GitHub API вернул HTTP %d: %s", response.StatusCode, detail)
	}

	decoder := json.NewDecoder(io.LimitReader(response.Body, 8<<20))
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("не удалось прочитать ответ GitHub API: %w", err)
	}
	return nil
}

func resolveCommitSHA(ref string) (string, error) {
	if strings.TrimSpace(ref) == "" {
		return "", errors.New("не указана ссылка на commit, тег или ветку")
	}
	var commit githubCommit
	if err := githubJSON("/commits/"+url.PathEscape(ref), &commit); err != nil {
		return "", err
	}
	sha := strings.ToLower(strings.TrimSpace(commit.SHA))
	if !isFullCommitSHA(sha) {
		return "", fmt.Errorf("GitHub вернул некорректный commit SHA для %q", ref)
	}
	return sha, nil
}

func resolveProjectVersion(mode, ref, enteredSHA string) (ProjectVersionResolution, error) {
	mode = strings.TrimSpace(strings.ToLower(mode))
	ref = strings.TrimSpace(ref)
	enteredSHA = strings.TrimSpace(enteredSHA)

	switch mode {
	case "release":
		var release githubRelease
		var err error
		if ref == "" {
			err = githubJSON("/releases/latest", &release)
		} else {
			err = githubJSON("/releases/tags/"+url.PathEscape(ref), &release)
		}
		if err != nil {
			return ProjectVersionResolution{}, fmt.Errorf("не удалось получить опубликованный стабильный релиз: %w", err)
		}
		if release.Draft || release.Prerelease || strings.TrimSpace(release.TagName) == "" {
			return ProjectVersionResolution{}, errors.New("выбранная запись не является опубликованным стабильным релизом")
		}
		ref = release.TagName
		sha, err := resolveCommitSHA(ref)
		if err != nil {
			return ProjectVersionResolution{}, fmt.Errorf("не удалось разрешить тег релиза %s: %w", ref, err)
		}
		if enteredSHA != "" && (!isFullCommitSHA(enteredSHA) || !strings.EqualFold(enteredSHA, sha)) {
			return ProjectVersionResolution{}, errors.New("релиз изменился после отображения версии. Обновите список версий и повторите выбор")
		}
		return ProjectVersionResolution{
			Success: true, Message: "Стабильный релиз разрешён в конкретный commit SHA.",
			Mode: mode, Ref: ref, CommitSHA: sha, Label: "Стабильный релиз " + ref,
		}, nil

	case "tag":
		if ref == "" {
			return ProjectVersionResolution{}, errors.New("не выбран тег проекта")
		}
		var tags []githubTag
		if err := githubJSON("/tags?per_page=100", &tags); err != nil {
			return ProjectVersionResolution{}, fmt.Errorf("не удалось проверить опубликованные теги: %w", err)
		}
		found := false
		for _, tag := range tags {
			if tag.Name == ref {
				found = true
				break
			}
		}
		if !found {
			return ProjectVersionResolution{}, fmt.Errorf("тег %q не найден среди доступных опубликованных тегов", ref)
		}
		sha, err := resolveCommitSHA(ref)
		if err != nil {
			return ProjectVersionResolution{}, fmt.Errorf("не удалось разрешить тег %s: %w", ref, err)
		}
		if enteredSHA != "" && (!isFullCommitSHA(enteredSHA) || !strings.EqualFold(enteredSHA, sha)) {
			return ProjectVersionResolution{}, errors.New("тег указывает на другой commit, чем тот, что показан в интерфейсе. Обновите список тегов")
		}
		return ProjectVersionResolution{
			Success: true, Message: "Опубликованный тег разрешён в конкретный commit SHA.",
			Mode: mode, Ref: ref, CommitSHA: sha, Label: "Тег " + ref,
		}, nil

	case "dev":
		var branch githubBranch
		if err := githubJSON("/branches/dev", &branch); err != nil {
			return ProjectVersionResolution{}, fmt.Errorf("не удалось получить ветку dev: %w", err)
		}
		sha := strings.ToLower(strings.TrimSpace(branch.Commit.SHA))
		if !isFullCommitSHA(sha) {
			return ProjectVersionResolution{}, errors.New("GitHub вернул некорректный SHA ветки dev")
		}
		if enteredSHA != "" && (!isFullCommitSHA(enteredSHA) || !strings.EqualFold(enteredSHA, sha)) {
			return ProjectVersionResolution{}, errors.New("ветка dev изменилась после отображения версии. Обновите список версий и повторите выбор")
		}
		return ProjectVersionResolution{
			Success: true, Message: "Ветка dev закреплена на конкретном commit SHA.",
			Mode: mode, Ref: "dev", CommitSHA: sha, Label: "Ветка dev (нестабильная)",
		}, nil

	case "commit":
		if !isInputCommitSHA(ref) {
			return ProjectVersionResolution{}, errors.New("укажите commit SHA длиной от 7 до 40 шестнадцатеричных символов")
		}
		sha, err := resolveCommitSHA(ref)
		if err != nil {
			return ProjectVersionResolution{}, fmt.Errorf("указанный commit не удалось проверить: %w", err)
		}
		if enteredSHA != "" && (!isFullCommitSHA(enteredSHA) || !strings.EqualFold(enteredSHA, sha)) {
			return ProjectVersionResolution{}, errors.New("commit SHA разрешился в другое значение. Проверьте ввод и повторите")
		}
		shortSHA := sha
		if len(shortSHA) > 12 {
			shortSHA = shortSHA[:12]
		}
		return ProjectVersionResolution{
			Success: true, Message: "Commit SHA проверен и закреплён.",
			Mode: mode, Ref: sha, CommitSHA: sha, Label: "Commit " + shortSHA,
		}, nil
	default:
		return ProjectVersionResolution{}, fmt.Errorf("неизвестный источник версии %q", mode)
	}
}

// GetProjectVersions returns published releases, Git tags, the current dev tip,
// and the locally installed version without changing any files.
func (a *App) GetProjectVersions() ProjectVersionsResult {
	result := ProjectVersionsResult{
		Tags:      []ProjectVersionChoice{},
		Installed: getProjectInstallState(projectInstallDirName),
	}
	var release githubRelease
	if err := githubJSON("/releases/latest", &release); err != nil {
		result.Message = "Не удалось загрузить список релизов: " + err.Error()
		return result
	}
	if release.Draft || release.Prerelease || release.TagName == "" {
		result.Message = "GitHub не вернул опубликованный стабильный релиз."
		return result
	}

	var tags []githubTag
	if err := githubJSON("/tags?per_page=100", &tags); err != nil {
		result.Message = "Не удалось загрузить опубликованные теги: " + err.Error()
		return result
	}
	var branch githubBranch
	if err := githubJSON("/branches/dev", &branch); err != nil {
		result.Message = "Не удалось получить commit ветки dev: " + err.Error()
		return result
	}
	devSHA := strings.ToLower(strings.TrimSpace(branch.Commit.SHA))
	if !isFullCommitSHA(devSHA) {
		result.Message = "GitHub вернул некорректный commit SHA ветки dev."
		return result
	}

	latestSHA := ""
	for _, tag := range tags {
		if tag.Name == release.TagName && isFullCommitSHA(tag.Commit.SHA) {
			latestSHA = strings.ToLower(tag.Commit.SHA)
			break
		}
	}
	if latestSHA == "" {
		var err error
		latestSHA, err = resolveCommitSHA(release.TagName)
		if err != nil {
			result.Message = "Не удалось разрешить commit последнего стабильного релиза: " + err.Error()
			return result
		}
	}

	result.LatestRelease = ProjectVersionChoice{
		Mode: "release", Ref: release.TagName, CommitSHA: latestSHA,
		Label: "Последний стабильный релиз — " + release.TagName, PublishedAt: release.PublishedAt,
	}
	result.Dev = ProjectVersionChoice{
		Mode: "dev", Ref: "dev", CommitSHA: devSHA,
		Label: "Ветка dev — последние изменения (нестабильно)",
	}

	for _, tag := range tags {
		name := strings.TrimSpace(tag.Name)
		sha := strings.ToLower(strings.TrimSpace(tag.Commit.SHA))
		if name == "" || !isFullCommitSHA(sha) {
			continue
		}
		result.Tags = append(result.Tags, ProjectVersionChoice{
			Mode: "tag", Ref: name, CommitSHA: sha, Label: name,
		})
	}
	result.Success = true
	result.Message = "Список опубликованных версий получен."
	return result
}

// ResolveProjectVersion previews and verifies a source before the download begins.
// expectedSHA is optional for preview calls; installation always requires it.
func (a *App) ResolveProjectVersion(mode, ref, expectedSHA string) ProjectVersionResolution {
	resolution, err := resolveProjectVersion(mode, ref, expectedSHA)
	if err != nil {
		return ProjectVersionResolution{Success: false, Message: err.Error(), Mode: mode, Ref: ref}
	}
	return resolution
}

// InstallProject downloads a resolved commit into a staging directory, verifies
// the archive and Ansible Git task, then switches the project directory with rollback.
func (a *App) InstallProject(mode, ref, expectedSHA string) CheckResult {
	targetDir, err := filepath.Abs(projectInstallDirName)
	if err != nil {
		return CheckResult{Success: false, Message: "Не удалось определить каталог проекта: " + err.Error()}
	}
	state := getProjectInstallState(targetDir)
	if state.Exists && !state.KnownVersion {
		return CheckResult{
			Success: false,
			Message: "Обнаружен существующий каталог ServeHub-2-dev без корректных метаданных версии. Он не изменён. Сделайте резервную копию и вручную переименуйте или переместите этот каталог, затем перезапустите установщик. Автоматически приписывать ему версию небезопасно.",
		}
	}

	// A repeat launch with the already recorded full SHA is a no-op and works
	// even when GitHub is temporarily unavailable.
	if mode == "commit" && state.KnownVersion &&
		isFullCommitSHA(ref) && isFullCommitSHA(expectedSHA) &&
		strings.EqualFold(ref, state.Metadata.CommitSHA) &&
		strings.EqualFold(expectedSHA, state.Metadata.CommitSHA) {
		if err := verifyProjectDirectory(targetDir); err == nil {
			return CheckResult{
				Success: true,
				Message: fmt.Sprintf("Уже установлена закреплённая версия %s (commit %s). Проект не изменён.", state.Metadata.DisplayName, state.Metadata.CommitSHA),
			}
		}
	}

	resolution, err := resolveProjectVersion(mode, ref, expectedSHA)
	if err != nil {
		return CheckResult{Success: false, Message: "Не удалось проверить выбранную версию: " + err.Error()}
	}
	if !isFullCommitSHA(expectedSHA) || !strings.EqualFold(expectedSHA, resolution.CommitSHA) {
		return CheckResult{
			Success: false,
			Message: "Установка остановлена: выбранный источник не совпадает с показанным commit SHA. Обновите список версий и повторите выбор.",
		}
	}

	if state.KnownVersion && strings.EqualFold(state.Metadata.CommitSHA, resolution.CommitSHA) {
		if err := verifyProjectDirectory(targetDir); err == nil {
			return CheckResult{
				Success: true,
				Message: fmt.Sprintf("Уже установлена выбранная версия %s (commit %s). Никакие файлы не заменялись.", state.Metadata.DisplayName, resolution.CommitSHA),
			}
		}
	}

	parentDir := filepath.Dir(targetDir)
	stageDir, err := os.MkdirTemp(parentDir, ".servehub-stage-")
	if err != nil {
		return CheckResult{Success: false, Message: "Не удалось создать временный каталог установки: " + err.Error()}
	}
	defer os.RemoveAll(stageDir)

	archivePath := filepath.Join(stageDir, "project.zip")
	if err := downloadProjectArchive(resolution.CommitSHA, archivePath); err != nil {
		return CheckResult{Success: false, Message: "Не удалось загрузить архив проекта: " + err.Error()}
	}

	projectDir := filepath.Join(stageDir, "project")
	if err := os.Mkdir(projectDir, 0755); err != nil {
		return CheckResult{Success: false, Message: "Не удалось создать временный каталог проекта: " + err.Error()}
	}
	if err := extractProjectArchive(archivePath, projectDir); err != nil {
		return CheckResult{Success: false, Message: "Архив проекта не прошёл проверку: " + err.Error()}
	}
	if err := verifyProjectDirectory(projectDir); err != nil {
		return CheckResult{Success: false, Message: "Загруженный проект не прошёл проверку структуры: " + err.Error()}
	}

	deployPath := filepath.Join(projectDir, "ansible", "deploy.yml")
	compatibilityPatch, err := pinAnsibleRepositoryVersion(deployPath)
	if err != nil {
		return CheckResult{Success: false, Message: "Не удалось подтвердить закрепление версии для серверов: " + err.Error()}
	}

	if state.KnownVersion {
		oldSecrets := filepath.Join(targetDir, "ansible", "vars", "secrets.yml")
		newSecrets := filepath.Join(projectDir, "ansible", "vars", "secrets.yml")
		if err := copySecretsIfPresent(oldSecrets, newSecrets); err != nil {
			return CheckResult{Success: false, Message: "Не удалось безопасно сохранить существующий secrets.yml: " + err.Error()}
		}
	}

	sourceRef := resolution.Ref
	if mode == "commit" {
		sourceRef = resolution.CommitSHA
	}
	metadata := ProjectVersionMetadata{
		SchemaVersion:      1,
		SourceMode:         strings.ToLower(strings.TrimSpace(mode)),
		SourceRef:          sourceRef,
		ResolvedRef:        resolution.Ref,
		CommitSHA:          resolution.CommitSHA,
		DisplayName:        resolution.Label,
		InstalledAt:        time.Now().UTC().Format(time.RFC3339),
		CompatibilityPatch: compatibilityPatch,
	}
	if err := writeProjectMetadata(projectDir, metadata); err != nil {
		return CheckResult{Success: false, Message: "Не удалось сохранить метаданные установленной версии: " + err.Error()}
	}
	if err := verifyProjectDirectory(projectDir); err != nil {
		return CheckResult{Success: false, Message: "Финальная проверка проекта не пройдена: " + err.Error()}
	}

	backupDir := ""
	if state.Exists {
		placeholder, err := os.MkdirTemp(parentDir, ".servehub-previous-")
		if err != nil {
			return CheckResult{Success: false, Message: "Не удалось подготовить безопасное переключение каталога: " + err.Error()}
		}
		if err := os.Remove(placeholder); err != nil {
			return CheckResult{Success: false, Message: "Не удалось подготовить резервный путь для прежней версии: " + err.Error()}
		}
		backupDir = placeholder
		if err := os.Rename(targetDir, backupDir); err != nil {
			return CheckResult{Success: false, Message: "Не удалось переместить прежнюю версию в резервный каталог: " + err.Error()}
		}
	}
	if err := os.Rename(projectDir, targetDir); err != nil {
		if backupDir != "" {
			if restoreErr := os.Rename(backupDir, targetDir); restoreErr != nil {
				return CheckResult{
					Success: false,
					Message: fmt.Sprintf("Не удалось установить новую версию (%v) и восстановить прежний каталог (%v). Прежняя версия сохранена по пути %s.", err, restoreErr, backupDir),
				}
			}
		}
		return CheckResult{Success: false, Message: "Не удалось переключить каталог на проверенную версию: " + err.Error()}
	}

	message := fmt.Sprintf("Установлен источник: %s. Закреплённый commit SHA: %s.", resolution.Label, resolution.CommitSHA)
	if compatibilityPatch != "" {
		message += " Совместимость старого Ansible playbook проверена: " + compatibilityPatch
	}
	if backupDir != "" {
		if err := os.RemoveAll(backupDir); err != nil {
			message += " Предыдущий каталог сохранён для ручного удаления: " + backupDir
		}
	}
	return CheckResult{Success: true, Message: message}
}

func downloadProjectArchive(commitSHA, destination string) error {
	if !isFullCommitSHA(commitSHA) {
		return errors.New("попытка загрузки с некорректным commit SHA заблокирована")
	}
	requestURL := "https://codeload.github.com/canntstand/ServeHub-2/zip/" + commitSHA
	request, err := http.NewRequest(http.MethodGet, requestURL, nil)
	if err != nil {
		return err
	}
	request.Header.Set("User-Agent", "ServeHub-2-Installer")
	client := &http.Client{Timeout: 3 * time.Minute}
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
		detail := strings.TrimSpace(string(body))
		if detail == "" {
			detail = response.Status
		}
		return fmt.Errorf("GitHub вернул HTTP %d: %s", response.StatusCode, detail)
	}
	if response.ContentLength > maxProjectArchiveSize {
		return fmt.Errorf("размер архива превышает лимит %d МиБ", maxProjectArchiveSize>>20)
	}

	file, err := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	written, copyErr := io.Copy(file, io.LimitReader(response.Body, maxProjectArchiveSize+1))
	closeErr := file.Close()
	if copyErr != nil {
		return copyErr
	}
	if closeErr != nil {
		return closeErr
	}
	if written <= 0 {
		return errors.New("GitHub вернул пустой архив")
	}
	if written > maxProjectArchiveSize {
		return fmt.Errorf("размер архива превысил лимит %d МиБ", maxProjectArchiveSize>>20)
	}
	return nil
}

func extractProjectArchive(archivePath, destination string) error {
	archiveFile, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer archiveFile.Close()
	info, err := archiveFile.Stat()
	if err != nil {
		return err
	}
	reader, err := zip.NewReader(archiveFile, info.Size())
	if err != nil {
		return fmt.Errorf("некорректный ZIP: %w", err)
	}
	if len(reader.File) == 0 || len(reader.File) > maxProjectArchiveFiles {
		return fmt.Errorf("недопустимое количество файлов в архиве: %d", len(reader.File))
	}

	root := ""
	seen := make(map[string]struct{}, len(reader.File))
	var totalExpanded uint64
	for _, entry := range reader.File {
		name := strings.ReplaceAll(entry.Name, "\\", "/")
		name = strings.TrimSuffix(name, "/")
		if name == "" || strings.ContainsRune(name, 0) || strings.HasPrefix(name, "/") {
			return fmt.Errorf("недопустимый путь в ZIP: %q", entry.Name)
		}
		if path.Clean(name) != name || name == "." || name == ".." || strings.HasPrefix(name, "../") {
			return fmt.Errorf("путь выходит за пределы каталога проекта: %q", entry.Name)
		}
		parts := strings.Split(name, "/")
		if len(parts) < 1 {
			return fmt.Errorf("недопустимый путь в ZIP: %q", entry.Name)
		}
		for _, part := range parts {
			if part == "" || part == "." || part == ".." || strings.ContainsAny(part, "<>:\"|?*") {
				return fmt.Errorf("недопустимый компонент пути в ZIP: %q", entry.Name)
			}
		}
		if root == "" {
			root = parts[0]
			if !strings.HasPrefix(root, "ServeHub-2-") || len(root) <= len("ServeHub-2-") {
				return fmt.Errorf("неожиданный корневой каталог архива: %q", root)
			}
		} else if parts[0] != root {
			return fmt.Errorf("архив содержит несколько корневых каталогов: %q", entry.Name)
		}
		if len(parts) == 1 {
			if !entry.FileInfo().IsDir() {
				return fmt.Errorf("файл находится вне каталога проекта: %q", entry.Name)
			}
			continue
		}
		if entry.Mode()&os.ModeSymlink != 0 || (!entry.FileInfo().IsDir() && !entry.Mode().IsRegular()) {
			return fmt.Errorf("символические ссылки и специальные файлы в архиве запрещены: %q", entry.Name)
		}
		if entry.UncompressedSize64 > maxExpandedProjectSize || totalExpanded > maxExpandedProjectSize-entry.UncompressedSize64 {
			return fmt.Errorf("распакованный архив превышает лимит %d МиБ", maxExpandedProjectSize>>20)
		}
		totalExpanded += entry.UncompressedSize64

		relative := strings.Join(parts[1:], "/")
		if relative == "" || path.Clean(relative) != relative || path.IsAbs(relative) ||
			relative == ".." || strings.HasPrefix(relative, "../") {
			return fmt.Errorf("небезопасный относительный путь в ZIP: %q", entry.Name)
		}
		key := strings.ToLower(relative)
		if _, exists := seen[key]; exists {
			return fmt.Errorf("повторяющийся путь в ZIP: %q", entry.Name)
		}
		seen[key] = struct{}{}

		localRelative := filepath.FromSlash(relative)
		target := filepath.Join(destination, localRelative)
		relCheck, err := filepath.Rel(destination, target)
		if err != nil || !filepath.IsLocal(relCheck) {
			return fmt.Errorf("путь выходит за пределы временного каталога: %q", entry.Name)
		}

		if entry.FileInfo().IsDir() || strings.HasSuffix(entry.Name, "/") {
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		mode := entry.Mode().Perm() & 0755
		if mode == 0 {
			mode = 0644
		}
		output, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
		if err != nil {
			return err
		}
		input, err := entry.Open()
		if err != nil {
			output.Close()
			return err
		}
		copied, copyErr := io.Copy(output, io.LimitReader(input, int64(entry.UncompressedSize64)+1))
		inputCloseErr := input.Close()
		outputCloseErr := output.Close()
		if copyErr != nil {
			return copyErr
		}
		if inputCloseErr != nil {
			return inputCloseErr
		}
		if outputCloseErr != nil {
			return outputCloseErr
		}
		if uint64(copied) != entry.UncompressedSize64 {
			return fmt.Errorf("размер распакованного файла не совпадает с ZIP: %q", entry.Name)
		}
	}
	if root == "" {
		return errors.New("не удалось определить корневой каталог архива")
	}
	return verifyProjectDirectory(destination)
}

func verifyProjectDirectory(projectDir string) error {
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
		info, err := os.Lstat(filepath.Join(projectDir, filepath.FromSlash(relative)))
		if err != nil {
			return fmt.Errorf("отсутствует обязательный файл %s", relative)
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("обязательный путь %s не является обычным файлом", relative)
		}
	}
	return nil
}

// pinAnsibleRepositoryVersion patches only the Git task that checks out the
// ServeHub-2 repository. This supports older tags whose playbook omitted the
// version field or hard-coded "dev"; unknown task layouts are rejected.
func pinAnsibleRepositoryVersion(deployPath string) (string, error) {
	content, err := os.ReadFile(deployPath)
	if err != nil {
		return "", err
	}
	updated, note, err := pinServeHubGitVersion(string(content))
	if err != nil {
		return "", err
	}
	if updated == string(content) {
		return note, nil
	}
	temporary := deployPath + ".servehub-version.tmp"
	file, err := os.OpenFile(temporary, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		return "", err
	}
	_, writeErr := file.WriteString(updated)
	closeErr := file.Close()
	if writeErr != nil {
		os.Remove(temporary)
		return "", writeErr
	}
	if closeErr != nil {
		os.Remove(temporary)
		return "", closeErr
	}
	if err := os.Rename(temporary, deployPath); err != nil {
		os.Remove(temporary)
		return "", err
	}
	return note, nil
}

func pinServeHubGitVersion(content string) (string, string, error) {
	lines := strings.Split(content, "\n")
	type candidate struct {
		start    int
		end      int
		repoLine int
		versionLine int
	}
	var candidates []candidate

	for index, line := range lines {
		match := gitModuleRegexp.FindStringSubmatch(line)
		if match == nil {
			continue
		}
		indent := len(match[1])
		end := index + 1
		for end < len(lines) {
			current := lines[end]
			trimmed := strings.TrimSpace(current)
			if trimmed == "" || strings.HasPrefix(trimmed, "#") {
				end++
				continue
			}
			if leadingWhitespace(current) <= indent {
				break
			}
			end++
		}

		repoLine := -1
		destMatches := false
		repoMatches := false
		versionLine := -1
		for lineIndex := index + 1; lineIndex < end; lineIndex++ {
			if repoMatch := yamlRepoRegexp.FindStringSubmatch(lines[lineIndex]); repoMatch != nil {
				repoLine = lineIndex
				repoValue := strings.Trim(strings.TrimSpace(repoMatch[1]), "\"'")
				repoMatches = strings.TrimSuffix(repoValue, ".git") == "https://github.com/canntstand/ServeHub-2"
			}
			if destMatch := yamlDestRegexp.FindStringSubmatch(lines[lineIndex]); destMatch != nil {
				destValue := strings.Trim(strings.TrimSpace(destMatch[1]), "\"'")
				destMatches = strings.HasSuffix(destValue, "/ServeHub-2")
			}
			if yamlVersionRegexp.MatchString(lines[lineIndex]) {
				versionLine = lineIndex
			}
		}
		if repoMatches && destMatches && repoLine >= 0 {
			candidates = append(candidates, candidate{
				start: index, end: end, repoLine: repoLine, versionLine: versionLine,
			})
		}
	}
	if len(candidates) != 1 {
		return "", "", fmt.Errorf("ожидалась ровно одна Git-задача для https://github.com/canntstand/ServeHub-2 с назначением /home/<deploy_user>/ServeHub-2; найдено: %d", len(candidates))
	}
	selected := candidates[0]
	if selected.versionLine >= 0 {
		versionMatch := yamlVersionRegexp.FindStringSubmatch(lines[selected.versionLine])
		if versionMatch != nil && strings.Contains(versionMatch[2], "servehub_version") {
			return content, "", nil
		}
		if versionMatch != nil {
			lines[selected.versionLine] = versionMatch[1] + "version: \"{{ servehub_version }}\""
			return strings.Join(lines, "\n"), "Жёстко заданная версия Git заменена на выбранный commit SHA.", nil
		}
	}
	repoIndent := lines[selected.repoLine][:len(lines[selected.repoLine])-len(strings.TrimLeft(lines[selected.repoLine], " \t"))]
	insertAt := selected.repoLine + 1
	lines = append(lines[:insertAt], append([]string{repoIndent + "version: \"{{ servehub_version }}\""}, lines[insertAt:]...)...)
	return strings.Join(lines, "\n"), "В старом Ansible playbook отсутствовало закрепление Git-версии; установщик добавил servehub_version в единственную проверенную Git-задачу.", nil
}

func leadingWhitespace(value string) int {
	for index, char := range value {
		if char != ' ' && char != '\t' {
			return index
		}
	}
	return len(value)
}

func copySecretsIfPresent(source, destination string) error {
	info, err := os.Lstat(source)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return errors.New("исходный secrets.yml не является обычным файлом")
	}
	content, err := os.ReadFile(source)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0700); err != nil {
		return err
	}
	if err := os.WriteFile(destination, content, 0600); err != nil {
		return err
	}
	return os.Chmod(destination, 0600)
}

func writeProjectMetadata(projectDir string, metadata ProjectVersionMetadata) error {
	content, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return err
	}
	content = append(content, '\n')
	metadataPath := filepath.Join(projectDir, ".servehub-version.json")
	temporary := metadataPath + ".tmp"
	file, err := os.OpenFile(temporary, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		return err
	}
	_, writeErr := file.Write(content)
	closeErr := file.Close()
	if writeErr != nil {
		os.Remove(temporary)
		return writeErr
	}
	if closeErr != nil {
		os.Remove(temporary)
		return closeErr
	}
	return os.Rename(temporary, metadataPath)
}

func readInstalledProjectMetadata(projectDir string) (ProjectVersionMetadata, error) {
	state := getProjectInstallState(projectDir)
	if !state.Exists {
		return ProjectVersionMetadata{}, errors.New("каталог проекта не найден")
	}
	if !state.KnownVersion {
		return ProjectVersionMetadata{}, errors.New(state.Message)
	}
	return state.Metadata, nil
}

func getProjectInstallState(projectDir string) ProjectInstallState {
	state := ProjectInstallState{}
	info, err := os.Lstat(projectDir)
	if os.IsNotExist(err) {
		state.Message = "Проект ещё не установлен."
		return state
	}
	state.Exists = true
	if err != nil {
		state.Message = "Не удалось проверить каталог проекта: " + err.Error()
		return state
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		state.Message = "Путь установки существует, но не является обычным каталогом."
		return state
	}
	metadataPath := filepath.Join(projectDir, ".servehub-version.json")
	metadataInfo, err := os.Lstat(metadataPath)
	if err != nil {
		state.Message = "Каталог проекта не содержит метаданных закреплённой версии. Автоматическое переключение запрещено."
		return state
	}
	if metadataInfo.Mode()&os.ModeSymlink != 0 || !metadataInfo.Mode().IsRegular() || metadataInfo.Size() > 64<<10 {
		state.Message = "Метаданные версии отсутствуют или имеют небезопасный формат."
		return state
	}
	content, err := os.ReadFile(metadataPath)
	if err != nil {
		state.Message = "Не удалось прочитать метаданные версии: " + err.Error()
		return state
	}
	var metadata ProjectVersionMetadata
	if err := json.Unmarshal(content, &metadata); err != nil {
		state.Message = "Файл метаданных версии повреждён. Автоматическое переключение запрещено."
		return state
	}
	if metadata.SchemaVersion != 1 || !isFullCommitSHA(metadata.CommitSHA) ||
		strings.TrimSpace(metadata.DisplayName) == "" ||
		!validSourceMode(metadata.SourceMode) {
		state.Message = "Метаданные версии не прошли проверку. Автоматическое переключение запрещено."
		return state
	}
	metadata.CommitSHA = strings.ToLower(metadata.CommitSHA)
	state.KnownVersion = true
	state.Metadata = metadata
	state.Message = fmt.Sprintf("Установлена версия %s, commit %s.", metadata.DisplayName, metadata.CommitSHA)
	return state
}

func validSourceMode(mode string) bool {
	switch strings.ToLower(mode) {
	case "release", "tag", "dev", "commit":
		return true
	default:
		return false
	}
}

func ansibleComposeNeedsBuild(composePath string) (bool, error) {
	content, err := os.ReadFile(composePath)
	if err != nil {
		return false, err
	}
	lines := strings.Split(string(content), "\n")
	inAnsible := false
	serviceIndent := -1
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		indent := leadingWhitespace(line)
		if !inAnsible {
			if trimmed == "ansible:" {
				inAnsible = true
				serviceIndent = indent
			}
			continue
		}
		if indent <= serviceIndent {
			break
		}
		if regexp.MustCompile("^[ \t]*build:[ \t]*$").MatchString(line) {
			return true, nil
		}
	}
	if !inAnsible {
		return false, errors.New("в compose-файле не найден сервис ansible")
	}
	return false, nil
}
