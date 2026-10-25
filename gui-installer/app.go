package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"

	pty "github.com/aymanbagabas/go-pty"
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

var ansiEscape = regexp.MustCompile(
	"\x1b\\][^\x07\x1b]*(\x07|\x1b\\\\)" +
	"|\x1b\\[[0-9;?]*[a-zA-Z]" +
	"|\x1b[()][A-Za-z0-9]" +
	"|\x1b[=>78]",
)

type App struct {
	ctx       context.Context
	cmdStdin  io.Writer
	ptty      pty.Pty
	activeCmd *pty.Cmd
}

func stripAnsi(s string) string {
	return ansiEscape.ReplaceAllString(s, "")
}

func (a *App) SendEnter() CheckResult {
	if a.cmdStdin == nil {
		return CheckResult{Success: false, Message: "Процесс деплоя не запущен или не ожидает ввода"}
	}

	_, err := a.cmdStdin.Write([]byte("\r\n"))
	if err != nil {
		return CheckResult{Success: false, Message: fmt.Sprintf("Не удалось отправить Enter: %s", err.Error())}
	}

	return CheckResult{Success: true, Message: "Сигнал Enter успешно отправлен"}
}

func NewApp() *App {
	return &App{}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

type CheckResult struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Exists  bool   `json:"exists,omitempty"`
}

type SecretsResult struct {
	Success    bool              `json:"success"`
	Message    string            `json:"message"`
	Content    string            `json:"content"`
	Values     map[string]string `json:"values"`
	IsTemplate bool              `json:"isTemplate"`
}

func (a *App) CloseApp() {
	wailsRuntime.Quit(a.ctx)
}

func (a *App) CheckDocker() CheckResult {
	_, err := exec.Command("docker", "info").Output()
	if err != nil {
		return CheckResult{
			Success: false,
			Message: "Docker не запущен или не установлен!",
		}
	}

	_, err = exec.Command("docker", "compose", "version").Output()
	if err != nil {
		return CheckResult{
			Success: false,
			Message: "Docker Compose не найден в системе!",
		}
	}

	return CheckResult{
		Success: true,
		Message: "Docker запущен и готов к работе.",
	}
}

func (a *App) MinimizeApp() {
	wailsRuntime.WindowMinimise(a.ctx)
}

func (a *App) MaximizeApp() {
	wailsRuntime.WindowMaximise(a.ctx)
}

func (a *App) RunDeployment(option int, verbose bool) CheckResult {
	_, _, logSensitiveValues, _, secretsErr := loadSecretsSource()
	if secretsErr != nil {
		return CheckResult{Success: false, Message: "Не удалось прочитать secrets.yml для безопасной проверки перед деплоем."}
	}
	if err := validateSavedSecretsForDeployment(option); err != nil {
		message := "Проверка конфигурации перед Ansible не пройдена: " + redactSensitiveText(err.Error(), logSensitiveValues)
		return CheckResult{Success: false, Message: message}
	}
	metadata, err := readInstalledProjectMetadata(filepath.Join(".", projectInstallDirName))
	if err != nil {
		return CheckResult{Success: false, Message: redactSensitiveText("Не удалось определить закреплённую версию проекта: "+err.Error(), logSensitiveValues)}
	}
	if !isFullCommitSHA(metadata.CommitSHA) {
		return CheckResult{Success: false, Message: "В метаданных установленного проекта отсутствует корректный commit SHA. Повторный деплой заблокирован."}
	}

	projectDir := filepath.Join(".", projectInstallDirName)
	composeFile := filepath.Join(projectDir, "docker-compose.ansible.yaml")
	needsBuild, err := ansibleComposeNeedsBuild(composeFile)
	if err != nil {
		return CheckResult{Success: false, Message: redactSensitiveText("Не удалось проверить Ansible Compose: "+err.Error(), logSensitiveValues)}
	}

	var steps []string
	debugArgs := ""
	if verbose {
		debugArgs = " -vvv"
	}
	versionArgs := " -e servehub_version=" + metadata.CommitSHA

	switch option {
	case 1:
		steps = []string{
			fmt.Sprintf("ansible-playbook -i ansible/inventory.ini ansible/deploy.yml --limit vps,local --tags bootstrap%s%s", debugArgs, versionArgs),
			fmt.Sprintf("ansible-playbook -i ansible/inventory.ini ansible/deploy.yml --limit vps --skip-tags bootstrap%s%s", debugArgs, versionArgs),
			fmt.Sprintf("ansible-playbook -i ansible/inventory.ini ansible/deploy.yml --limit local --skip-tags bootstrap%s%s", debugArgs, versionArgs),
		}
	case 2:
		steps = []string{
			fmt.Sprintf("ansible-playbook -i ansible/inventory.ini ansible/deploy.yml --limit vps --skip-tags bootstrap%s%s", debugArgs, versionArgs),
			fmt.Sprintf("ansible-playbook -i ansible/inventory.ini ansible/deploy.yml --limit local --skip-tags bootstrap%s%s", debugArgs, versionArgs),
		}
	case 3:
		steps = []string{
			fmt.Sprintf("ansible-playbook -i ansible/inventory.ini ansible/deploy.yml --limit local --tags bootstrap%s%s", debugArgs, versionArgs),
			fmt.Sprintf("ansible-playbook -i ansible/inventory.ini ansible/deploy.yml --limit local --skip-tags bootstrap%s%s", debugArgs, versionArgs),
		}
	case 4:
		steps = []string{
			fmt.Sprintf("ansible-playbook -i ansible/inventory.ini ansible/deploy.yml --limit local --skip-tags bootstrap%s%s", debugArgs, versionArgs),
		}
	case 5:
		steps = []string{
			fmt.Sprintf("ansible-playbook -i ansible/inventory.ini ansible/deploy.yml --limit vps --tags bootstrap%s%s", debugArgs, versionArgs),
			fmt.Sprintf("ansible-playbook -i ansible/inventory.ini ansible/deploy.yml --limit vps --skip-tags bootstrap%s%s", debugArgs, versionArgs),
		}
	case 6:
		steps = []string{
			fmt.Sprintf("ansible-playbook -i ansible/inventory.ini ansible/deploy.yml --limit vps --skip-tags bootstrap%s%s", debugArgs, versionArgs),
		}
	default:
		return CheckResult{Success: false, Message: "Неизвестный сценарий развертывания!"}
	}

	for _, ansibleCmd := range steps {
		wailsRuntime.EventsEmit(a.ctx, "deploy-log", fmt.Sprintf("Закреплённая версия ServeHub-2: %s\n", metadata.CommitSHA))

		cmdArgs := []string{"compose", "-f", "./" + filepath.ToSlash(composeFile), "run"}
		if needsBuild {
			cmdArgs = append(cmdArgs, "--build")
		}
		cmdArgs = append(cmdArgs, "--rm", "ansible", "sh", "-c", ansibleCmd)

		ptty, err := pty.New()
		if err != nil {
			return CheckResult{Success: false, Message: redactSensitiveText(fmt.Sprintf("Не удалось создать псевдотерминал: %s", err.Error()), logSensitiveValues)}
		}
		_ = ptty.Resize(200, 50)

		cmd := ptty.CommandContext(a.ctx, "docker", cmdArgs...)
		cmd.Env = append(os.Environ(), "TERM=xterm-256color")
		if err := cmd.Start(); err != nil {
			ptty.Close()
			return CheckResult{Success: false, Message: redactSensitiveText(fmt.Sprintf("Не удалось запустить Docker: %s", err.Error()), logSensitiveValues)}
		}

		a.ptty = ptty
		a.cmdStdin = ptty
		a.activeCmd = cmd

		go func(p pty.Pty) {
			scanner := bufio.NewScanner(p)
			scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
			for scanner.Scan() {
				clean := redactSensitiveText(stripAnsi(scanner.Text()), logSensitiveValues)
				if clean == "" {
					continue
				}
				wailsRuntime.EventsEmit(a.ctx, "deploy-log", clean+"\n")
			}
		}(ptty)

		waitErr := cmd.Wait()
		ptty.Close()
		a.ptty = nil
		a.cmdStdin = nil
		a.activeCmd = nil

		if waitErr != nil {
			return CheckResult{
				Success: false,
				Message: redactSensitiveText(fmt.Sprintf("Ansible завершился с ошибкой: %s", waitErr.Error()), logSensitiveValues),
			}
		}
	}

	return CheckResult{
		Success: true,
		Message: redactSensitiveText(fmt.Sprintf("Все этапы деплоя Ansible успешно выполнены для commit %s.", metadata.CommitSHA), logSensitiveValues),
	}
}

