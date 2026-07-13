package main

import (
	"archive/zip"
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
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
}

type SecretsResult struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Content string `json:"content"`
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

func (a *App) InstallProject() CheckResult {
	if _, err := os.Stat("ServeHub-2-main"); err == nil {
		return CheckResult{
			Success: true,
			Message: "Проект уже был развернут ранее (папка ServeHub-2-main существует).",
		}
	}

	url := "https://github.com/canntstand/ServeHub-2/archive/refs/heads/main.zip"

	resp, err := http.Get(url)
	if err != nil {
		return CheckResult{Success: false, Message: "Не удалось подключиться к GitHub: " + err.Error()}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return CheckResult{Success: false, Message: "GitHub вернул ошибку: " + resp.Status}
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return CheckResult{Success: false, Message: "Ошибка при чтении данных архива: " + err.Error()}
	}

	zipReader, err := zip.NewReader(bytes.NewReader(bodyBytes), int64(len(bodyBytes)))
	if err != nil {
		return CheckResult{Success: false, Message: "Не удалось прочитать скачанный zip-архив: " + err.Error()}
	}

	destFolder := "."

	for _, f := range zipReader.File {
		fpath := filepath.Join(destFolder, f.Name)

		if f.FileInfo().IsDir() {
			os.MkdirAll(fpath, os.ModePerm)
			continue
		}

		if err = os.MkdirAll(filepath.Dir(fpath), os.ModePerm); err != nil {
			return CheckResult{Success: false, Message: "Не удалось создать структуру папок: " + err.Error()}
		}

		outFile, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return CheckResult{Success: false, Message: "Не удалось создать файл на диске: " + err.Error()}
		}

		rc, err := f.Open()
		if err != nil {
			outFile.Close()
			return CheckResult{Success: false, Message: "Ошибка чтения файла из архива: " + err.Error()}
		}

		_, err = io.Copy(outFile, rc)

		outFile.Close()
		rc.Close()

		if err != nil {
			return CheckResult{Success: false, Message: "Ошибка записи файла на диск: " + err.Error()}
		}
	}

	return CheckResult{
		Success: true,
		Message: "ServeHub-2 успешно скачан и развернут в рабочей папке!",
	}
}

func (a *App) SaveSecrets(yamlContent string) CheckResult {
	targetDir := filepath.Join(".", "ServeHub-2-main", "ansible", "vars")

	err := os.MkdirAll(targetDir, os.ModePerm)
	if err != nil {
		return CheckResult{Success: false, Message: "Не удалось создать директорию конфигурации: " + err.Error()}
	}

	targetFile := filepath.Join(targetDir, "secrets.yml")

	err = os.WriteFile(targetFile, []byte(yamlContent), 0644)
	if err != nil {
		return CheckResult{Success: false, Message: "Не удалось записать файл secrets.yml: " + err.Error()}
	}

	return CheckResult{Success: true, Message: "Файл secrets.yml успешно создан!"}
}

func (a *App) MinimizeApp() {
	wailsRuntime.WindowMinimise(a.ctx)
}

func (a *App) MaximizeApp() {
	wailsRuntime.WindowMaximise(a.ctx)
}

func (a *App) RunDeployment(option int, verbose bool) CheckResult {
	var steps []string
	debugArgs := ""
	if verbose {
		debugArgs = " -vvv"
	}

	switch option {
	case 1:
		steps = []string{
			fmt.Sprintf("ansible-playbook -i ansible/inventory.ini ansible/deploy.yml --limit vps,local --tags bootstrap%s", debugArgs),
			fmt.Sprintf("ansible-playbook -i ansible/inventory.ini ansible/deploy.yml --limit vps --skip-tags bootstrap%s", debugArgs),
			fmt.Sprintf("ansible-playbook -i ansible/inventory.ini ansible/deploy.yml --limit local --skip-tags bootstrap%s", debugArgs),
		}
	case 2:
		steps = []string{
			fmt.Sprintf("ansible-playbook -i ansible/inventory.ini ansible/deploy.yml --limit vps --skip-tags bootstrap%s", debugArgs),
			fmt.Sprintf("ansible-playbook -i ansible/inventory.ini ansible/deploy.yml --limit local --skip-tags bootstrap%s", debugArgs),
		}
	case 3:
		steps = []string{
			fmt.Sprintf("ansible-playbook -i ansible/inventory.ini ansible/deploy.yml --limit local --tags bootstrap%s", debugArgs),
			fmt.Sprintf("ansible-playbook -i ansible/inventory.ini ansible/deploy.yml --limit local --skip-tags bootstrap%s", debugArgs),
		}
	case 4:
		steps = []string{
			fmt.Sprintf("ansible-playbook -i ansible/inventory.ini ansible/deploy.yml --limit local --skip-tags bootstrap%s", debugArgs),
		}
	case 5:
		steps = []string{
			fmt.Sprintf("ansible-playbook -i ansible/inventory.ini ansible/deploy.yml --limit vps --tags bootstrap%s", debugArgs),
			fmt.Sprintf("ansible-playbook -i ansible/inventory.ini ansible/deploy.yml --limit vps --skip-tags bootstrap%s", debugArgs),
		}
	case 6:
		steps = []string{
			fmt.Sprintf(" ansible-playbook -i ansible/inventory.ini ansible/deploy.yml --limit vps --skip-tags bootstrap%s", debugArgs),
		}
	default:
		return CheckResult{Success: false, Message: "Неизвестный сценарий развертывания!"}
	}

	for _, ansibleCmd := range steps {
    wailsRuntime.EventsEmit(a.ctx, "deploy-log")

    cmdArgs := []string{
        "compose",
        "-f", "./ServeHub-2-main/docker-compose.ansible.yaml",
        "run", "--rm", "ansible",
        "sh", "-c", ansibleCmd,
    }

    ptty, err := pty.New()
    if err != nil {
        return CheckResult{Success: false, Message: fmt.Sprintf("Не удалось создать псевдотерминал: %s", err.Error())}
    }
    _ = ptty.Resize(200, 50)

    cmd := ptty.CommandContext(a.ctx, "docker", cmdArgs...)
    cmd.Env = append(os.Environ(), "TERM=xterm-256color")

    if err := cmd.Start(); err != nil {
        ptty.Close()
        return CheckResult{Success: false, Message: fmt.Sprintf("Не удалось запустить Docker: %s", err.Error())}
    }

    a.ptty = ptty
    a.cmdStdin = ptty
    a.activeCmd = cmd

    go func(p pty.Pty) {
        scanner := bufio.NewScanner(p)
        scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
        for scanner.Scan() {
            clean := stripAnsi(scanner.Text())
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

    if waitErr != nil {
        return CheckResult{
            Success: false,
            Message: fmt.Sprintf("Ansible завершился с ошибкой: %s", waitErr.Error()),
        }
    }
}

	return CheckResult{
		Success: true,
		Message: "Все этапы деплоя Ansible успешно выполнены!",
	}
}

func (a *App) CheckSecrets() CheckResult {
	targetFile := filepath.Join(".", "ServeHub-2-main", "ansible", "vars", "secrets.yml")

	if _, err := os.Stat(targetFile); err == nil {
		return CheckResult{
			Success: true,
			Message: "Обнаружен ранее созданный конфигурационный файл secrets.yml.",
		}
	}

	return CheckResult{
		Success: false,
		Message: "Файл конфигурации не найден.",
	}
}

func (a *App) LoadSecrets() SecretsResult {
	targetFile := filepath.Join(".", "ServeHub-2-main", "ansible", "vars", "secrets.yml")

	data, err := os.ReadFile(targetFile)
	if err != nil {
		return SecretsResult{
			Success: false,
			Message: "Не удалось прочитать файл secrets.yml: " + err.Error(),
		}
	}

	return SecretsResult{
		Success: true,
		Message: "Файл secrets.yml успешно загружен.",
		Content: string(data),
	}
}