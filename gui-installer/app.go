package main

import (
	"context"
	"os/exec"
	"archive/zip"
	"bytes"
	"io"
	"fmt"
	"bufio"
	"net/http"
	"os"
	"path/filepath"
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

type App struct {
	ctx context.Context
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
			fmt.Sprintf("ansible-playbook -i ansible/inventory.ini ansible/deploy.yml --limit vps --skip-tags bootstrap%s", debugArgs),
		}
	default:
		return CheckResult{Success: false, Message: "Неизвестный сценарий развертывания!"}
	}

	for i, ansibleCmd := range steps {
		statusMsg := fmt.Sprintf("\n[ СИСТЕМА ]: Запуск шага %d из %d...\nКоманда: %s\n--------------------------------------------------\n", i+1, len(steps), ansibleCmd)
		wailsRuntime.EventsEmit(a.ctx, "deploy-log", statusMsg)

		cmdArgs := []string{
			"compose",
			"-f", "./ServeHub-2-main/docker-compose.ansible.yaml",
			"run", "-T", "--rm", "ansible",
			"sh", "-c", ansibleCmd,
		}

		cmd := exec.CommandContext(a.ctx, "docker", cmdArgs...)

		stdoutPipe, err := cmd.StdoutPipe()
		if err != nil {
			return CheckResult{Success: false, Message: fmt.Sprintf("Ошибка инициализации потока вывода на шаге %d: %s", i+1, err.Error())}
		}
		cmd.Stderr = cmd.Stdout

		if err := cmd.Start(); err != nil {
			return CheckResult{Success: false, Message: fmt.Sprintf("Не удалось запустить Docker на шаге %d: %s", i+1, err.Error())}
		}

		scanner := bufio.NewScanner(stdoutPipe)
		for scanner.Scan() {
			wailsRuntime.EventsEmit(a.ctx, "deploy-log", scanner.Text()+"\n")
		}

		if err := cmd.Wait(); err != nil {
			return CheckResult{
				Success: false,
				Message: fmt.Sprintf("Ansible завершился с ошибкой: %s", err.Error()),
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