package main

import (
	"context"
	"os/exec"
	"archive/zip"
	"bytes"
	"io"
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