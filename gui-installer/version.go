package main

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
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
	"sync"
	"time"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
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
	IntegritySHA256    string `json:"integrity_sha256,omitempty"`
}

type ProjectInstallState struct {
	Directory    string                 `json:"directory"`
	Exists       bool                   `json:"exists"`
	Complete     bool                   `json:"complete"`
	Partial      bool                   `json:"partial"`
	KnownVersion     bool                   `json:"knownVersion"`
	IntegrityVerified bool                  `json:"integrityVerified"`
	Metadata         ProjectVersionMetadata `json:"metadata"`
	Message      string                 `json:"message"`
}

type ProjectInstallDirectoryResult struct {
	Success   bool                `json:"success"`
	Directory string              `json:"directory"`
	Installed ProjectInstallState `json:"installed"`
	Message   string              `json:"message"`
}

type installerPreferences struct {
	ProjectDirectory string `json:"project_directory"`
}

var projectInstallMu sync.Mutex

type ProjectVersionsResult struct {
	Success          bool                   `json:"success"`
	Message          string                 `json:"message"`
	InstallDirectory string                 `json:"installDirectory"`
	LatestRelease    ProjectVersionChoice   `json:"latestRelease"`
	Tags             []ProjectVersionChoice `json:"tags"`
	Dev              ProjectVersionChoice   `json:"dev"`
	Installed        ProjectInstallState    `json:"installed"`
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

func preferencesFilePath() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("не удалось определить каталог настроек пользователя: %w", err)
	}
	if strings.TrimSpace(configDir) == "" {
		return "", errors.New("системный каталог настроек пользователя пуст")
	}
	return filepath.Join(configDir, "ServeHub-2", "installer.json"), nil
}

func defaultProjectInstallDirectory() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("не удалось определить домашний каталог пользователя: %w", err)
	}
	if strings.TrimSpace(home) == "" {
		return "", errors.New("домашний каталог пользователя пуст")
	}
	return filepath.Join(home, "ServeHub-2"), nil
}

func normalizeProjectInstallDirectory(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", errors.New("укажите каталог установки ServeHub-2")
	}
	if value == "~" || strings.HasPrefix(value, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		if value == "~" {
			value = home
		} else {
			value = filepath.Join(home, filepath.FromSlash(strings.TrimPrefix(value, "~/")))
		}
	}
	absolute, err := filepath.Abs(value)
	if err != nil {
		return "", fmt.Errorf("не удалось разрешить путь установки: %w", err)
	}
	absolute = filepath.Clean(absolute)
	if filepath.Dir(absolute) == absolute {
		return "", errors.New("корень файловой системы нельзя использовать как каталог установки")
	}
	if _, err := os.Lstat(filepath.Join(absolute, "gui-installer", "go.mod")); err == nil {
		if _, gitErr := os.Lstat(filepath.Join(absolute, ".git")); gitErr == nil {
			return "", errors.New("нельзя использовать корень исходного Git-репозитория как каталог установки")
		}
	}
	info, err := os.Lstat(absolute)
	if err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			return "", errors.New("каталог установки не должен быть символической ссылкой")
		}
		if !info.IsDir() {
			return "", errors.New("путь установки уже существует, но не является каталогом")
		}
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("не удалось проверить путь установки: %w", err)
	}
	return absolute, nil
}

func saveProjectInstallDirectory(value string) error {
	absolute, err := normalizeProjectInstallDirectory(value)
	if err != nil {
		return err
	}
	configPath, err := preferencesFilePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(configPath), 0700); err != nil {
		return fmt.Errorf("не удалось создать каталог настроек установщика: %w", err)
	}
	content, err := json.MarshalIndent(installerPreferences{ProjectDirectory: absolute}, "", "  ")
	if err != nil {
		return fmt.Errorf("не удалось сериализовать настройки установщика: %w", err)
	}
	content = append(content, '\n')
	temporary := configPath + ".tmp"
	_ = os.Remove(temporary)
	if err := os.WriteFile(temporary, content, 0600); err != nil {
		return fmt.Errorf("не удалось записать временные настройки установщика: %w", err)
	}
	if err := os.Chmod(temporary, 0600); err != nil {
		_ = os.Remove(temporary)
		return fmt.Errorf("не удалось ограничить права настроек установщика: %w", err)
	}
	if err := os.Rename(temporary, configPath); err != nil {
		_ = os.Remove(temporary)
		return fmt.Errorf("не удалось сохранить настройки установщика: %w", err)
	}
	return nil
}

// configuredProjectInstallDirectory reads the persisted path first. A cwd-based
// path is only a one-time compatibility fallback for older installer versions.
func configuredProjectInstallDirectory() (string, error) {
	configPath, err := preferencesFilePath()
	if err != nil {
		return "", err
	}
	content, err := os.ReadFile(configPath)
	if err == nil {
		if len(content) > 64<<10 {
			return "", errors.New("файл настроек установщика превышает допустимый размер")
		}
		var prefs installerPreferences
		if err := json.Unmarshal(content, &prefs); err != nil {
			return "", errors.New("файл настроек установщика повреждён; выберите каталог установки заново")
		}
		return normalizeProjectInstallDirectory(prefs.ProjectDirectory)
	}
	if !os.IsNotExist(err) {
		return "", fmt.Errorf("не удалось прочитать настройки установщика: %w", err)
	}

	candidates := []string{}
	if cwd, cwdErr := os.Getwd(); cwdErr == nil {
		candidates = append(candidates, filepath.Join(cwd, projectInstallDirName))
	}
	if home, homeErr := os.UserHomeDir(); homeErr == nil {
		candidates = append(candidates, filepath.Join(home, projectInstallDirName))
	}
	for _, candidate := range candidates {
		info, statErr := os.Lstat(candidate)
		if statErr == nil && info.Mode()&os.ModeSymlink == 0 && info.IsDir() {
			absolute, absErr := filepath.Abs(candidate)
			if absErr == nil {
				return filepath.Clean(absolute), nil
			}
		}
	}
	return defaultProjectInstallDirectory()
}

func projectInstallDirectoryResult(directory string) ProjectInstallDirectoryResult {
	state := getProjectInstallState(directory)
	state.Directory = directory
	return ProjectInstallDirectoryResult{
		Success: true, Directory: directory, Installed: state, Message: state.Message,
	}
}

func (a *App) GetProjectInstallDirectory() ProjectInstallDirectoryResult {
	directory, err := configuredProjectInstallDirectory()
	if err != nil {
		return ProjectInstallDirectoryResult{Success: false, Message: err.Error()}
	}
	configPath, pathErr := preferencesFilePath()
	if pathErr == nil {
		if _, statErr := os.Lstat(configPath); os.IsNotExist(statErr) {
			if saveErr := saveProjectInstallDirectory(directory); saveErr != nil {
				return ProjectInstallDirectoryResult{Success: false, Directory: directory, Installed: getProjectInstallState(directory), Message: saveErr.Error()}
			}
		}
	}
	return projectInstallDirectoryResult(directory)
}

func (a *App) InspectProjectInstallDirectory(directory string) ProjectInstallDirectoryResult {
	absolute, err := normalizeProjectInstallDirectory(directory)
	if err != nil {
		return ProjectInstallDirectoryResult{Success: false, Message: err.Error()}
	}
	result := projectInstallDirectoryResult(absolute)
	if !result.Installed.Exists {
		result.Message = "Каталог доступен для новой установки: " + absolute
	} else {
		result.Message = result.Installed.Message + " Путь: " + absolute
	}
	return result
}

func (a *App) SetProjectInstallDirectory(directory string) ProjectInstallDirectoryResult {
	absolute, err := normalizeProjectInstallDirectory(directory)
	if err != nil {
		return ProjectInstallDirectoryResult{Success: false, Message: err.Error()}
	}
	if err := saveProjectInstallDirectory(absolute); err != nil {
		return ProjectInstallDirectoryResult{Success: false, Directory: absolute, Installed: getProjectInstallState(absolute), Message: err.Error()}
	}
	return projectInstallDirectoryResult(absolute)
}

func (a *App) ChooseProjectInstallDirectory() ProjectInstallDirectoryResult {
	current, _ := configuredProjectInstallDirectory()
	options := wailsRuntime.OpenDialogOptions{
		Title: "Выберите каталог установки ServeHub-2",
		DefaultDirectory: current,
	}
	selected, err := wailsRuntime.OpenDirectoryDialog(a.ctx, options)
	if err != nil {
		return ProjectInstallDirectoryResult{Success: false, Message: "Не удалось открыть выбор каталога: " + err.Error()}
	}
	if strings.TrimSpace(selected) == "" {
		return ProjectInstallDirectoryResult{Success: false, Message: "Выбор каталога отменён."}
	}
	return a.SetProjectInstallDirectory(selected)
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
	directory, directoryErr := configuredProjectInstallDirectory()
	if directoryErr != nil {
		return ProjectVersionsResult{Success: false, Message: directoryErr.Error(), Tags: []ProjectVersionChoice{}}
	}
	result := ProjectVersionsResult{
		Tags: []ProjectVersionChoice{},
		InstallDirectory: directory,
		Installed: getProjectInstallState(directory),
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
func (a *App) PerformProjectAction(action, mode, ref, expectedSHA string, confirmReplace, replaceLegacy bool) CheckResult {
	targetDir, err := configuredProjectInstallDirectory()
	if err != nil {
		return CheckResult{Success: false, Message: "Не удалось определить каталог установки: " + err.Error()}
	}

	switch strings.ToLower(strings.TrimSpace(action)) {
	case "use":
		projectInstallMu.Lock()
		defer projectInstallMu.Unlock()
		state := getProjectInstallState(targetDir)
		if !state.Exists || !state.Complete || !state.KnownVersion || !state.IntegrityVerified {
			return CheckResult{Success: false, Message: "Текущая установка не подтверждена как полная версия ServeHub-2. Выберите установку или восстановление неполной установки."}
		}
		if err := verifyProjectDirectory(targetDir); err != nil {
			return CheckResult{Success: false, Message: "Текущая установка не прошла повторную проверку: " + err.Error()}
		}
		return CheckResult{
			Success: true, Exists: true,
			Message: fmt.Sprintf("Используется текущая версия %s (commit %s). Каталог и версия не изменялись: %s", state.Metadata.DisplayName, state.Metadata.CommitSHA, targetDir),
		}
	case "recover":
		return a.RecoverProjectInstallation()
	case "install", "update":
		ctx, finish, err := a.beginProjectInstallContext()
		if err != nil {
			return CheckResult{Success: false, Message: err.Error()}
		}
		defer finish()
		return installProjectVersion(ctx, targetDir, action, mode, ref, expectedSHA, confirmReplace, replaceLegacy)
	default:
		return CheckResult{Success: false, Message: "Неизвестное действие с установкой проекта."}
	}
}

// InstallProject remains as a compatibility wrapper for existing Wails bindings.
// A path containing an installation is never overwritten by the install action.
func (a *App) InstallProject(mode, ref, expectedSHA string, replaceLegacy bool) CheckResult {
	targetDir, err := configuredProjectInstallDirectory()
	if err != nil {
		return CheckResult{Success: false, Message: "Не удалось определить каталог установки: " + err.Error()}
	}
	ctx, finish, err := a.beginProjectInstallContext()
	if err != nil {
		return CheckResult{Success: false, Message: err.Error()}
	}
	defer finish()
	return installProjectVersion(ctx, targetDir, "install", mode, ref, expectedSHA, false, replaceLegacy)
}

func (a *App) beginProjectInstallContext() (context.Context, func(), error) {
	base := a.ctx
	if base == nil {
		base = context.Background()
	}
	ctx, cancel := context.WithCancel(base)
	a.installContextMu.Lock()
	if a.installCancel != nil {
		a.installContextMu.Unlock()
		cancel()
		return nil, nil, errors.New("установщик уже выполняет операцию. Дождитесь её завершения или отмените загрузку.")
	}
	a.installCancel = cancel
	a.installContextMu.Unlock()

	finish := func() {
		cancel()
		a.installContextMu.Lock()
		a.installCancel = nil
		a.installContextMu.Unlock()
	}
	return ctx, finish, nil
}

func (a *App) CancelProjectInstallation() CheckResult {
	a.installContextMu.Lock()
	cancel := a.installCancel
	a.installContextMu.Unlock()
	if cancel == nil {
		return CheckResult{Success: false, Message: "Сейчас нет активной загрузки для отмены."}
	}
	cancel()
	return CheckResult{Success: true, Message: "Запрошена отмена загрузки. Текущая установленная версия не будет изменена."}
}

func (a *App) RecoverProjectInstallation() CheckResult {
	targetDir, err := configuredProjectInstallDirectory()
	if err != nil {
		return CheckResult{Success: false, Message: "Не удалось определить каталог установки: " + err.Error()}
	}
	projectInstallMu.Lock()
	defer projectInstallMu.Unlock()

	parentDir := filepath.Dir(targetDir)
	cleaned, cleanupErr := removeStaleProjectStages(parentDir)
	if cleanupErr != nil {
		return CheckResult{Success: false, Message: "Не удалось очистить временные каталоги загрузки: " + cleanupErr.Error()}
	}

	state := getProjectInstallState(targetDir)
	quarantined := ""
	if state.Exists && state.Partial {
		quarantined, err = uniqueSiblingPath(parentDir, ".servehub-incomplete-")
		if err != nil {
			return CheckResult{Success: false, Message: "Не удалось подготовить каталог для сохранения неполной установки: " + err.Error()}
		}
		if err := os.Rename(targetDir, quarantined); err != nil {
			return CheckResult{Success: false, Message: "Не удалось переместить неполную установку в безопасный карантин: " + err.Error()}
		}
	}
	message := fmt.Sprintf("Восстановление завершено. Удалено временных каталогов: %d.", cleaned)
	if quarantined != "" {
		message += " Неполный каталог сохранён, а не удалён: " + quarantined
	}
	if cleaned == 0 && quarantined == "" {
		message += " Неполные установки и оставшиеся временные данные не обнаружены."
	}
	message += " Теперь можно выбрать установку версии из списка."
	return CheckResult{Success: true, Message: message}
}

func installProjectVersion(ctx context.Context, requestedDir, action, mode, ref, expectedSHA string, confirmReplace, replaceLegacy bool) CheckResult {
	projectInstallMu.Lock()
	defer projectInstallMu.Unlock()

	targetDir, err := normalizeProjectInstallDirectory(requestedDir)
	if err != nil {
		return CheckResult{Success: false, Message: "Некорректный каталог установки: " + err.Error()}
	}
	state := getProjectInstallState(targetDir)
	if state.Partial {
		return CheckResult{Success: false, Message: "В выбранном каталоге обнаружена неполная установка. Сначала выполните «Восстановить неполную установку»; каталог будет сохранён отдельно."}
	}
	if action == "install" && state.Exists {
		return CheckResult{Success: false, Message: "Каталог установки уже существует. Выберите «Использовать текущую установку» или «Обновить/переключить версию»; автоматическая перезапись запрещена."}
	}
	if action == "update" && !state.Exists {
		return CheckResult{Success: false, Message: "В выбранном каталоге нет установки для обновления. Выберите действие «Установить выбранную версию»."}
	}

	legacyReplacement := state.Exists && state.Complete && !state.KnownVersion
	if legacyReplacement && !replaceLegacy {
		return CheckResult{Success: false, Message: "В каталоге есть проект без проверяемых метаданных версии. Подтвердите сохранение всей старой папки и миграцию конфигурации перед переключением."}
	}
	if action == "update" && state.Exists && state.KnownVersion &&
		isFullCommitSHA(expectedSHA) && !strings.EqualFold(state.Metadata.CommitSHA, expectedSHA) && !confirmReplace {
		return CheckResult{Success: false, Message: "Для обновления или смены версии сначала явно подтвердите замену кода. Текущая версия и пользовательские файлы будут сохранены в резервной копии."}
	}
	if state.Exists && state.KnownVersion && isFullCommitSHA(expectedSHA) &&
		strings.EqualFold(state.Metadata.CommitSHA, expectedSHA) {
		if err := verifyProjectDirectory(targetDir); err == nil {
			return CheckResult{
				Success: true, Exists: true,
				Message: fmt.Sprintf("Выбрана уже установленная версия %s (commit %s). Повторная загрузка и замена файлов не выполнялись: %s", state.Metadata.DisplayName, state.Metadata.CommitSHA, targetDir),
			}
		}
	}

	resolution, err := resolveProjectVersion(mode, ref, expectedSHA)
	if err != nil {
		return CheckResult{Success: false, Message: "Не удалось проверить выбранную версию: " + err.Error()}
	}
	if !isFullCommitSHA(expectedSHA) || !strings.EqualFold(expectedSHA, resolution.CommitSHA) {
		return CheckResult{Success: false, Message: "Установка остановлена: выбранный источник не совпадает с показанным commit SHA. Обновите список версий и повторите выбор."}
	}

	parentDir := filepath.Dir(targetDir)
	if err := os.MkdirAll(parentDir, 0700); err != nil {
		return CheckResult{Success: false, Message: installationErrorMessage("подготовка каталога", err)}
	}
	stageDir, err := os.MkdirTemp(parentDir, ".servehub-stage-")
	if err != nil {
		return CheckResult{Success: false, Message: installationErrorMessage("создание временного каталога", err)}
	}
	defer os.RemoveAll(stageDir)

	archivePath := filepath.Join(stageDir, "project.zip")
	if err := downloadProjectArchiveContext(ctx, resolution.CommitSHA, archivePath); err != nil {
		return CheckResult{Success: false, Message: "Не удалось загрузить архив проекта: " + installationErrorMessage("загрузка", err) + " Текущая установка не изменялась."}
	}

	projectDir := filepath.Join(stageDir, "project")
	if err := os.Mkdir(projectDir, 0755); err != nil {
		return CheckResult{Success: false, Message: "Не удалось создать временный каталог проекта: " + installationErrorMessage("распаковка", err)}
	}
	if err := extractProjectArchive(archivePath, projectDir); err != nil {
		return CheckResult{Success: false, Message: "Архив проекта не прошёл проверку: " + installationErrorMessage("распаковка", err) + " Временные данные будут очищены; текущая установка не изменялась."}
	}
	if err := verifyProjectDirectory(projectDir); err != nil {
		return CheckResult{Success: false, Message: "Загруженный проект не прошёл проверку структуры: " + err.Error()}
	}

	deployPath := filepath.Join(projectDir, "ansible", "deploy.yml")
	compatibilityPatch, err := pinAnsibleRepositoryVersion(deployPath)
	if err != nil {
		return CheckResult{Success: false, Message: "Не удалось подтвердить закрепление версии для серверов: " + err.Error()}
	}

	if state.Exists && state.Complete && (action == "update" || legacyReplacement) {
		if err := preserveUserProjectFiles(targetDir, projectDir); err != nil {
			return CheckResult{Success: false, Message: "Не удалось сохранить пользовательскую конфигурацию: " + err.Error() + ". Исходная установка не изменялась."}
		}
	}

	integritySHA256, err := projectIntegrityFingerprint(projectDir)
	if err != nil {
		return CheckResult{Success: false, Message: "Не удалось вычислить контрольную сумму установленного кода: " + err.Error()}
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
		IntegritySHA256:    integritySHA256,
	}
	if err := writeProjectMetadata(projectDir, metadata); err != nil {
		return CheckResult{Success: false, Message: "Не удалось сохранить метаданные установленной версии: " + err.Error()}
	}
	if err := verifyProjectDirectory(projectDir); err != nil {
		return CheckResult{Success: false, Message: "Финальная проверка проекта не пройдена: " + err.Error()}
	}
	if ctx.Err() != nil {
		return CheckResult{Success: false, Message: installationErrorMessage("переключение версии", ctx.Err()) + " Текущая установленная версия не изменялась."}
	}

	backupDir := ""
	if state.Exists {
		backupDir, err = uniqueSiblingPath(parentDir, ".servehub-backup-")
		if err != nil {
			return CheckResult{Success: false, Message: "Не удалось подготовить резервную копию текущего каталога: " + err.Error()}
		}
		if err := os.Rename(targetDir, backupDir); err != nil {
			return CheckResult{Success: false, Message: "Не удалось сохранить прежнюю версию в резервную копию: " + installationErrorMessage("резервное копирование", err)}
		}
	}
	if err := os.Rename(projectDir, targetDir); err != nil {
		if backupDir != "" {
			if restoreErr := os.Rename(backupDir, targetDir); restoreErr != nil {
				return CheckResult{Success: false, Message: fmt.Sprintf("Не удалось переключить каталог на проверенную версию (%v) и восстановить прежний каталог (%v). Прежняя версия сохранена в резервной копии: %s", err, restoreErr, backupDir)}
			}
		}
		return CheckResult{Success: false, Message: "Не удалось переключить каталог на проверенную версию: " + installationErrorMessage("переключение каталога", err)}
	}

	installedState := getProjectInstallState(targetDir)
	if !installedState.Complete || !installedState.KnownVersion || !installedState.IntegrityVerified ||
		!strings.EqualFold(installedState.Metadata.CommitSHA, resolution.CommitSHA) {
		quarantine, quarantineErr := uniqueSiblingPath(parentDir, ".servehub-unverified-")
		if quarantineErr == nil {
			_ = os.Rename(targetDir, quarantine)
		}
		if backupDir != "" {
			if restoreErr := os.Rename(backupDir, targetDir); restoreErr != nil {
				return CheckResult{Success: false, Message: "Финальная проверка после переключения не пройдена; восстановить старую версию автоматически не удалось. Резервная копия: " + backupDir}
			}
		}
		return CheckResult{Success: false, Message: "Финальная проверка установленной версии не пройдена. Предыдущая версия восстановлена, если она существовала."}
	}

	message := fmt.Sprintf("Проверенная версия %s (commit %s) установлена в %s.", resolution.Label, resolution.CommitSHA, targetDir)
	if compatibilityPatch != "" {
		message += " Закрепление версии в Ansible проверено: " + compatibilityPatch
	}
	if backupDir != "" {
		message += " Предыдущий каталог целиком сохранён в резервной копии: " + backupDir + ". Он не удаляется автоматически."
	}
	return CheckResult{Success: true, Exists: true, Message: message}
}

func uniqueSiblingPath(parentDir, prefix string) (string, error) {
	if err := os.MkdirAll(parentDir, 0700); err != nil {
		return "", err
	}
	placeholder, err := os.MkdirTemp(parentDir, prefix)
	if err != nil {
		return "", err
	}
	if err := os.Remove(placeholder); err != nil {
		return "", err
	}
	return placeholder, nil
}

func removeStaleProjectStages(parentDir string) (int, error) {
	entries, err := os.ReadDir(parentDir)
	if os.IsNotExist(err) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	removed := 0
	for _, entry := range entries {
		if !strings.HasPrefix(entry.Name(), ".servehub-stage-") || entry.Type()&os.ModeSymlink != 0 {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return removed, err
		}
		if !info.IsDir() {
			continue
		}
		if err := os.RemoveAll(filepath.Join(parentDir, entry.Name())); err != nil {
			return removed, err
		}
		removed++
	}
	return removed, nil
}

func installationErrorMessage(stage string, err error) string {
	lower := strings.ToLower(err.Error())
	switch {
	case errors.Is(err, context.Canceled) || strings.Contains(lower, "context canceled"):
		return "Операция отменена пользователем или закрытием установщика; временные данные будут очищены."
	case strings.Contains(lower, "no space left on device") || strings.Contains(lower, "not enough space") ||
		strings.Contains(lower, "disk full") || strings.Contains(lower, "there is not enough space"):
		return "Недостаточно свободного места для " + stage + ". Освободите место и повторите попытку."
	case strings.Contains(lower, "timeout") || strings.Contains(lower, "deadline exceeded") ||
		strings.Contains(lower, "i/o timeout"):
		return "Истекло время ожидания при " + stage + ". Проверьте подключение к GitHub и повторите попытку."
	case strings.Contains(lower, "no such host") || strings.Contains(lower, "connection refused") ||
		strings.Contains(lower, "connection reset") || strings.Contains(lower, "network is unreachable") ||
		strings.Contains(lower, "unexpected eof") || strings.Contains(lower, "eof"):
		return "Соединение было прервано при " + stage + ". Проверьте сеть и доступность GitHub; текущая установленная версия не изменялась."
	default:
		return err.Error()
	}
}

func preserveUserProjectFiles(sourceDir, destinationDir string) error {
	for _, relative := range []string{
		"ansible/vars/secrets.yml",
		"ansible/inventory.ini",
		".env",
		"docker-compose.override.yaml",
	} {
		if err := copyRegularUserFile(sourceDir, destinationDir, relative, true); err != nil {
			return err
		}
	}

	return filepath.WalkDir(sourceDir, func(sourcePath string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(sourceDir, sourcePath)
		if err != nil {
			return err
		}
		if relative == "." {
			return nil
		}
		if entry.IsDir() {
			if entry.Name() == ".git" || strings.HasPrefix(entry.Name(), ".servehub-stage-") ||
				strings.HasPrefix(entry.Name(), ".servehub-backup-") {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() || relative == ".servehub-version.json" {
			return nil
		}
		for _, preserved := range []string{
			filepath.Join("ansible", "vars", "secrets.yml"),
			filepath.Join("ansible", "inventory.ini"),
			".env",
			"docker-compose.override.yaml",
		} {
			if relative == preserved {
				return nil
			}
		}
		destinationPath := filepath.Join(destinationDir, relative)
		if _, err := os.Lstat(destinationPath); err == nil {
			return nil
		} else if !os.IsNotExist(err) {
			return err
		}
		return copyRegularUserFile(sourceDir, destinationDir, relative, false)
	})
}

func copyRegularUserFile(sourceDir, destinationDir, relative string, replace bool) error {
	source := filepath.Join(sourceDir, relative)
	info, err := os.Lstat(source)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("не удалось проверить %s: %w", relative, err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return fmt.Errorf("пользовательский файл %s не является обычным файлом", relative)
	}
	destination := filepath.Join(destinationDir, relative)
	if !replace {
		if _, err := os.Lstat(destination); err == nil {
			return nil
		} else if !os.IsNotExist(err) {
			return err
		}
	}
	content, err := os.ReadFile(source)
	if err != nil {
		return fmt.Errorf("не удалось прочитать %s: %w", relative, err)
	}
	mode := info.Mode().Perm() & 0755
	if relative == filepath.Join("ansible", "vars", "secrets.yml") ||
		relative == filepath.Join("ansible", "inventory.ini") ||
		relative == ".env" || relative == "docker-compose.override.yaml" {
		mode = 0600
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0755); err != nil {
		return fmt.Errorf("не удалось создать каталог для %s: %w", relative, err)
	}
	if err := os.WriteFile(destination, content, mode); err != nil {
		return fmt.Errorf("не удалось перенести %s: %w", relative, err)
	}
	if mode == 0600 {
		if err := os.Chmod(destination, 0600); err != nil {
			return fmt.Errorf("не удалось ограничить права %s: %w", relative, err)
		}
	}
	return nil
}

func downloadProjectArchive(commitSHA, destination string) error {
	return downloadProjectArchiveContext(context.Background(), commitSHA, destination)
}

func downloadProjectArchiveContext(ctx context.Context, commitSHA, destination string) error {
	if !isFullCommitSHA(commitSHA) {
		return errors.New("попытка загрузки с некорректным commit SHA заблокирована")
	}
	requestURL := "https://codeload.github.com/canntstand/ServeHub-2/zip/" + commitSHA
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
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

var projectIntegrityFiles = []string{
	"docker-compose.ansible.yaml",
	"docker-compose.local.yaml",
	"docker-compose.remote.yaml",
	"ansible/deploy.yml",
	"ansible/tasks/bootstrap_os.yml",
	"ansible/tasks/setup_local.yml",
	"ansible/tasks/setup_remote.yml",
	"scripts/ansible-entrypoint.sh",
}

// projectIntegrityFingerprint binds a set of core project files to the installed
// version metadata. Configuration and user-managed files are intentionally
// excluded, so preserving local settings does not invalidate the code check.
func projectIntegrityFingerprint(projectDir string) (string, error) {
	hash := sha256.New()
	for _, relative := range projectIntegrityFiles {
		filePath := filepath.Join(projectDir, filepath.FromSlash(relative))
		info, err := os.Lstat(filePath)
		if err != nil {
			return "", fmt.Errorf("не удалось проверить %s: %w", relative, err)
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			return "", fmt.Errorf("ключевой файл %s не является обычным файлом", relative)
		}
		if _, err := io.WriteString(hash, relative+"\x00"); err != nil {
			return "", err
		}
		file, err := os.Open(filePath)
		if err != nil {
			return "", fmt.Errorf("не удалось открыть %s: %w", relative, err)
		}
		_, copyErr := io.Copy(hash, file)
		closeErr := file.Close()
		if copyErr != nil {
			return "", fmt.Errorf("не удалось проверить содержимое %s: %w", relative, copyErr)
		}
		if closeErr != nil {
			return "", fmt.Errorf("не удалось закрыть %s после проверки: %w", relative, closeErr)
		}
		if _, err := hash.Write([]byte{0}); err != nil {
			return "", err
		}
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
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
	state := ProjectInstallState{Directory: projectDir}
	info, err := os.Lstat(projectDir)
	if os.IsNotExist(err) {
		state.Message = "Проект ещё не установлен."
		return state
	}
	state.Exists = true
	if err != nil {
		state.Partial = true
		state.Message = "Не удалось проверить каталог проекта: " + err.Error()
		return state
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		state.Partial = true
		state.Message = "Путь установки существует, но не является обычным каталогом. Для безопасности он не будет изменён автоматически."
		return state
	}
	if err := verifyProjectDirectory(projectDir); err != nil {
		state.Partial = true
		state.Message = "Найдена неполная установка: " + err.Error()
		return state
	}
	state.Complete = true

	metadataPath := filepath.Join(projectDir, ".servehub-version.json")
	metadataInfo, err := os.Lstat(metadataPath)
	if os.IsNotExist(err) {
		state.Message = "Каталог содержит структуру ServeHub-2, но не содержит метаданных версии. Фактический commit установить нельзя."
		return state
	}
	if err != nil {
		state.Message = "Не удалось проверить метаданные версии: " + err.Error()
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
		state.Message = "Файл метаданных версии повреждён. Фактический commit установить нельзя."
		return state
	}
	if metadata.SchemaVersion != 1 || !isFullCommitSHA(metadata.CommitSHA) ||
		strings.TrimSpace(metadata.DisplayName) == "" ||
		!validSourceMode(metadata.SourceMode) {
		state.Message = "Метаданные версии не прошли проверку. Фактический commit установить нельзя."
		return state
	}
	metadata.CommitSHA = strings.ToLower(metadata.CommitSHA)
	state.Metadata = metadata
	if !regexp.MustCompile("^[a-f0-9]{64}$").MatchString(strings.ToLower(metadata.IntegritySHA256)) {
		state.Message = "Структура проекта существует, но метаданные не содержат контрольную сумму ключевых файлов. Фактическую версию нельзя подтвердить; переключение потребует явного подтверждения и резервной копии."
		return state
	}
	actualIntegrity, err := projectIntegrityFingerprint(projectDir)
	if err != nil {
		state.Message = "Не удалось проверить контрольную сумму ключевых файлов: " + err.Error()
		return state
	}
	if !strings.EqualFold(actualIntegrity, metadata.IntegritySHA256) {
		state.Message = "Контрольная сумма ключевых файлов не совпадает с метаданными. Код мог быть изменён; фактическая версия не подтверждена."
		return state
	}
	state.KnownVersion = true
	state.IntegrityVerified = true
	state.Message = fmt.Sprintf("Структура и контрольные суммы ключевых файлов проверены. В метаданных указан источник %s, commit %s.", metadata.DisplayName, metadata.CommitSHA)
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
