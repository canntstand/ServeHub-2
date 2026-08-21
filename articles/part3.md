# Установщик на Go Wails: Полный гайд. Своя self-hosted двухсерверная VPN-архитектура: ЧАСТЬ 3

Здесь пойдет речь полностью про разработку установщика на Go Wails v2.

Повествование будет идти таким образом: описание архитектуры в целом, потом код по страницам с картинками как это все выглядит. В отличие от черновика, в этом материале отображен весь рабочий код без сокращений с разбором каждой из страниц.

В Wails основными файлами, с которыми проходит 90% работы это `index.html`, `styles.css`, `main.js`, `app.go`. Остальные файлы мы либо не трогаем, либо настраиваем один раз в самом начале.

Для фронтенда я использовал стандартный html, css, js без современных фреймворков (так как я в них не разбираюсь). Frontend часть подробно рассматривать в этой статье не планируется, a html будет использоваться только чтобы показать, где используются js функции.

## Что вообще делает установщик?

1.  Пользователя встречает главная страница с описанием.
2.  Проверка запущен/установлен ли Docker (в нем запускается ansible).
3.  Копирование кода репозитория рядом с установщиком.
4.  Настройка `secrets.yml` для ansible (форма ввода, данные из которой потом записываются в виде `.yml` конфига).
5.  Проверка конфигурации: показывается как будет выглядеть сохраненный конфиг, также дается возможность сделать себе копию.
6.  Выбор деплоя (как себя будет вести Ansible). Делится на 3 варианта: для локального сервера, для удаленного сервера и на оба сервера сразу. И каждый из этих вариантов делится еще на 2: полный деплой (с первоначальной настройкой сервера), либо только разворачивание приложений если проект уже запускался.
7.  Подтверждение выбора и начало деплоя. Появляется окно с терминалом в котором пишутся логи ansible.
8.  Если деплой завершился ошибкой - кнопка выйти или запустить заново. Если деплой успешен - небольшой гайд что делать дальше и кнопка выхода.

## Экскурс в Wails (как работает)

Wails делится на Frontend и Backend часть. Основной его смысл в том, что он отрисовывает страницы как стандартный браузер и переводит выводы Go функций в js. Получается таким образом: в Go мы пишем основную логику, Wails переводит данные из Go к JS, который уже работает со статикой страниц.

## Первоначальная настройка

Про установку много рассказывать не буду, для работы понадобятся:

*   Go 1.21+
*   NPM / Node.js (15+)
*   Wails v2
*   libwebkit2gtk-4.1-dev — только для Linux (необходим для работы графического движка Wails)

Чтобы создать первоначальную структуру файлов в терминале нужно выполнить `wails init`.

Первый файл с которым предстоит работать - `main.go` (здесь мы создаем наше приложение, даем ему базовые настройки по типу размера окна, привязываем ассеты).

**main.go:**
```go
package main // Объявление основного пакета приложения

import (
	"embed" // Библиотека для внедрения статических файлов в бинарный код приложения

	"github.com/wailsapp/wails/v2" // Основной пакет фреймворка Wails
	"github.com/wailsapp/wails/v2/pkg/options" // Конфигурационные параметры приложения
	"github.com/wailsapp/wails/v2/pkg/options/assetserver" // Настройки сервера статических файлов
)

// Директива компилятора go:embed:
// Указывает компилятору включить содержимое папки 'frontend/dist' в скомпилированный файл.
// 'all:' гарантирует, что будут включены скрытые файлы (начинающиеся с точки) 
// и файлы с нижним подчеркиванием.
//
//go:embed all:frontend/dist
var assets embed.FS // Переменная 'assets' представляет собой виртуальную файловую систему, содержащую все упакованные ресурсы

func main() {
	// Создаем экземпляр структуры App (содержит логику методов Go, доступных из JS)
	app := NewApp()

	// Запуск приложения Wails с заданными параметрами конфигурации:
	err := wails.Run(&options.App{
		Title:     "ServeHub-2 Installer", // Заголовок окна приложения
		Width:     1024,                  // Ширина окна
		Height:    768,                   // Высота окна
		Frameless: true,                  // Отключение системной рамки для создания кастомного интерфейса
		AssetServer: &assetserver.Options{
			Assets: assets, // Привязываем внедренную файловую систему к серверу ассетов
		},
		OnStartup: app.startup, // Метод, вызываемый при запуске приложения для инициализации контекста
		Bind: []interface{}{ // Регистрация экземпляра App для вызова его методов из JavaScript
			app,
		},
	})

	// Если при запуске произошла критическая ошибка, выводим её в консоль
	if err != nil {
		println("Error:", err.Error())
	}
}
```

## Логика смены страниц

Изначально Wails - это SPA, т.е все происходит на одной странице, а javascript меняет ее наполнение, поэтому чтобы реализовать приложение с несколькими страницами я использовал механизм “шагов”.

**index.html** (Контейнер прогресс-бара):
```html
<!-- Контейнер для отображения прогресса установки -->
<div class="progress-container">
    <!-- 
      Область визуализации шагов. 
      Здесь должны находиться элементы с классом .step-dot, 
      которые JavaScript будет обновлять через функцию updateStepProgress.
    -->
    <div class="progress-bar-track">
        <div id="step-progress-fill" class="progress-bar-fill"></div>
        
        <!-- Пример точек прогресса, соответствующих 7 шагам -->
        <div class="step-dot" data-step="1"></div>
        <div class="step-dot" data-step="2"></div>
        <div class="step-dot" data-step="3"></div>
        <div class="step-dot" data-step="4"></div>
        <div class="step-dot" data-step="5"></div>
        <div class="step-dot" data-step="6"></div>
        <div class="step-dot" data-step="7"></div>
    </div>

    <!-- Текстовое описание текущего этапа -->
    <div class="step-label">
        Шаг <span id="step-progress-current">1</span> из 7 — 
        <span id="step-progress-title">Добро пожаловать</span>
    </div>
</div>
```

Для управления навигацией написаны соответствующие функции, которые связывают элементы HTML и логику `App.js`.

**main.js** (Глобальные функции и роутер):
```javascript
/**
 * Словарь названий шагов для отображения в прогресс-баре.
 */
const STEP_TITLES = {
    1: 'Добро пожаловать',
    2: 'Проверка требований',
    3: 'Установка компонентов',
    4: 'Настройка secrets.yml',
    5: 'Проверка конфигурации',
    6: 'Выбор сценария деплоя',
    7: 'Запуск деплоя',
};

/**
 * Обновляет визуальное состояние прогресс-бара и индикаторов шагов.
 * @param {number} stepNumber - Текущий номер шага.
 */
function updateStepProgress(stepNumber) {
    const totalSteps = 7;
    const fill = document.getElementById('step-progress-fill');
    const currentEl = document.getElementById('step-progress-current');
    const titleEl = document.getElementById('step-progress-title');

    // 1. Обновление ширины прогресс-бара (процент заполнения)
    if (fill) {
        // Вычисляем процент так, чтобы на 1-м шаге было 0%, а на последнем — 100%
        const pct = ((stepNumber - 1) / (totalSteps - 1)) * 100;
        fill.style.width = `${pct}%`;
    }
    
    // 2. Обновление текстовых индикаторов
    if (currentEl) currentEl.innerText = stepNumber;
    if (titleEl) titleEl.innerText = STEP_TITLES[stepNumber] || '';

    // 3. Обновление визуальных точек (dots) прогресса
    document.querySelectorAll('.step-dot').forEach((dot) => {
        const dotStep = parseInt(dot.getAttribute('data-step'), 10);
        dot.classList.remove('completed', 'active');
        
        // Подсвечиваем пройденные шаги как 'completed', текущий — как 'active'
        if (dotStep < stepNumber) dot.classList.add('completed');
        if (dotStep === stepNumber) dot.classList.add('active');
    });
}

/**
 * Главный переключатель экранов мастера установки.
 * Скрывает неактивные шаги и отображает целевой.
 * @param {number} stepNumber - Номер шага для перехода.
 */
window.goToStep = function(stepNumber) {
    // 1. Скрытие всех экранов шагов
    document.querySelectorAll('.step').forEach(step => {
        step.classList.remove('active');
    });

    // 2. Отображение целевого экрана
    const nextStep = document.getElementById(`step-${stepNumber}`);
    if (nextStep) {
        nextStep.classList.add('active');
    }

    // 3. Обновление прогресс-бара
    updateStepProgress(stepNumber);

    // 4. Триггеры автоматической логики:
    // При переходе на определенные шаги запускаем соответствующие скрипты (проверки или установку).
    if (stepNumber === 2) {
        runDockerCheck();
    }
    if (stepNumber === 3) {
        runInstallation();
    }
}
```

## Ревью по страницам

### Шаг 1: Добро пожаловать (SPA и Роутинг)

На первом экране отображается базовая информация и кнопка запуска. По нажатию вызывается глобальная функция `goToStep(2)`, инициирующая проверку окружения.

**HTML-разметка шага (index.html):**
```html
<!-- 
  Главный контейнер мастера установки ServeHub-2.
  Используется как точка входа для пользователя при первом запуске приложения.
-->
<div class="installer-container">
    <header class="installer-header">
        <h1>ServeHub-2</h1>
        <p class="subtitle">Мастер установки</p>
    </header>

    <main class="installer-content">
        <p>Добро пожаловать! Эта программа поможет вам установить и настроить проект <strong>ServeHub-2</strong> на вашем компьютере.</p>
    </main>

    <footer class="installer-actions">
        <!-- Кнопка перехода к следующему шагу установки -->
        <button id="btn-start" class="btn-primary" onclick="goToStep(2)">
            Продолжить
        </button>
        
        <!-- Ссылка на исходный код проекта для прозрачности и контроля -->
        <a href="https://github.com/canntstand/ServeHub-2" target="_blank" class="link-github">
            <img src="assets/github-icon.svg" alt="GitHub Logo" class="icon">
            GitHub Repository
        </a>
    </footer>
</div>
```

### Шаг 2: Проверка требований

Здесь происходит взаимодействие фронтенда с бэкендом для проверки наличия Docker. Для структурирования ответа в Go используется `CheckResult`.

**Бэкенд на Go (app.go):**
```go
// CheckResult определяет стандартный формат ответа для всех операций бэкенда.
// Поля помечены тегами json для корректной сериализации при передаче данных в JavaScript (через Wails).
type CheckResult struct {
    Success bool   `json:"success"` // Флаг успешности выполнения операции
    Message string `json:"message"` // Описание результата или текст ошибки
}

// CheckDocker проверяет наличие и работоспособность Docker и Docker Compose в хост-системе.
// Метод выполняет попытку вызова команд через os/exec и возвращает статус готовности.
func (a *App) CheckDocker() CheckResult {
    // 1. Проверка работоспособности Docker:
    // Выполняем `docker info` — эта команда возвращает системную информацию, если Docker запущен.
    // Если команда завершается с ошибкой, значит демон Docker недоступен или не установлен.
    _, err := exec.Command("docker", "info").Output()
    if err != nil {
        return CheckResult{
            Success: false, 
            Message: "Docker не запущен или не установлен!",
        }
    }

    // 2. Проверка наличия Docker Compose:
    // Docker Compose часто устанавливается как плагин или отдельный бинарный файл.
    // Вызов `docker compose version` подтверждает, что плагин доступен для использования.
    _, err = exec.Command("docker", "compose", "version").Output()
    if err != nil {
        return CheckResult{
            Success: false, 
            Message: "Docker Compose не найден в системе!",
        }
    }

    // 3. Успешный результат:
    // Обе зависимости успешно обнаружены и готовы к работе.
    return CheckResult{
        Success: true, 
        Message: "Docker запущен и готов к работе.",
    }
}
```

**Фронтенд (main.js):**
```javascript
/**
 * Выполняет проверку доступности Docker и Docker Compose в системе.
 * Используется на этапе инициализации приложения для подтверждения готовности окружения.
 */
window.runDockerCheck = async function() {
    // 1. Получение ссылок на DOM-элементы для обновления состояния интерфейса
    const statusText = document.getElementById('status-text');
    const spinner = document.getElementById('status-loading');
    const btnNext = document.getElementById('btn-step2-next');
    const btnRetry = document.getElementById('btn-retry');

    // 2. Настройка UI перед запуском проверки:
    // Включаем спиннер, блокируем кнопку перехода и сбрасываем состояние текста
    spinner.style.display = 'block';
    spinner.className = 'spinner';
    statusText.innerText = 'Проверяем Docker и Docker Compose...';
    statusText.style.color = '#9a9aa0'; // Нейтральный цвет во время ожидания
    btnNext.disabled = true;
    btnRetry.style.display = 'none';

    // 3. Имитация задержки (700 мс):
    // Позволяет пользователю заметить визуальную смену состояний, избегая "дерганья" интерфейса
    // при очень быстром выполнении проверки на бэкенде.
    await new Promise(resolve => setTimeout(resolve, 700));

    try {
        // 4. Вызов Go-функции CheckDocker через мост Wails:
        const result = await CheckDocker();
        spinner.style.display = 'none'; // Скрываем индикатор после получения ответа

        if (result.success) {
            // Если Docker найден и корректно работает
            statusText.innerText = `✓ ${result.message}`;
            statusText.style.color = '#818cf8'; // Акцентный цвет успеха
            btnNext.disabled = false; // Открываем путь к следующему шагу
        } else {
            // Если возникла логическая ошибка (например, Docker не запущен)
            statusText.innerText = `✗ Ошибка: ${result.message}.`;
            statusText.style.color = '#e5484d'; // Цвет опасности
            btnRetry.style.display = 'block'; // Предлагаем попробовать еще раз
        }
    } catch (err) {
        // 5. Обработка системных исключений (например, сбой соединения с бэкендом)
        spinner.style.display = 'none';
        statusText.innerText = 'Критическая ошибка при вызове проверки бэкенда:\n' + err;
        statusText.style.color = '#e5484d';
        btnRetry.style.display = 'block';
    }
}
```

### Шаг 3: Установка компонентов

На этом шаге скачивается полная архитектура проекта с GitHub и производится поиск уже существующего конфига.

**Загрузка архива на бэкенде (app.go):**
```go
// InstallProject скачивает исходный код проекта с GitHub и распаковывает его в рабочую директорию.
// Метод предотвращает повторную загрузку, если папка проекта уже существует.
func (a *App) InstallProject() CheckResult {
    // 1. Проверка существования проекта:
    // Проверяем наличие папки "ServeHub-2-main". Если она есть, предполагаем, 
    // что проект уже развернут, и прерываем выполнение, чтобы не перезаписывать данные.
    if _, err := os.Stat("ServeHub-2-main"); err == nil {
        return CheckResult{
            Success: true, 
            Message: "Проект уже был развернут ранее (папка ServeHub-2-main существует).",
        }
    }

    // 2. Загрузка архива:
    // Используем стандартный HTTP-клиент для получения ZIP-архива из репозитория.
    url := "https://github.com/canntstand/ServeHub-2/archive/refs/heads/main.zip"
    resp, err := http.Get(url)
    if err != nil {
        return CheckResult{Success: false, Message: "Не удалось подключиться к GitHub: " + err.Error()}
    }
    defer resp.Body.Close() // Гарантируем закрытие потока ответа

    // Проверяем, что сервер вернул статус 200 OK
    if resp.StatusCode != http.StatusOK {
        return CheckResult{Success: false, Message: "GitHub вернул ошибку: " + resp.Status}
    }

    // 3. Чтение содержимого в память:
    bodyBytes, err := io.ReadAll(resp.Body)
    if err != nil {
        return CheckResult{Success: false, Message: "Ошибка при чтении данных архива: " + err.Error()}
    }

    // Инициализируем ZIP-ридер из байтового массива
    zipReader, err := zip.NewReader(bytes.NewReader(bodyBytes), int64(len(bodyBytes)))
    if err != nil {
        return CheckResult{Success: false, Message: "Не удалось прочитать скачанный zip-архив: " + err.Error()}
    }

    destFolder := "." // Распаковка производится в текущую директорию приложения

    // 4. Итерация и распаковка файлов из архива:
    for _, f := range zipReader.File {
        fpath := filepath.Join(destFolder, f.Name)

        // Обработка директорий: создаем папку, если элемент архива является директорией
        if f.FileInfo().IsDir() {
            os.MkdirAll(fpath, os.ModePerm)
            continue
        }

        // Создаем родительскую структуру папок для файла
        if err = os.MkdirAll(filepath.Dir(fpath), os.ModePerm); err != nil {
            return CheckResult{Success: false, Message: "Не удалось создать структуру папок: " + err.Error()}
        }

        // Создаем файл на диске с правами доступа, указанными в ZIP-архиве
        outFile, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
        if err != nil {
            return CheckResult{Success: false, Message: "Не удалось создать файл на диске: " + err.Error()}
        }

        // Открываем поток файла внутри архива
        rc, err := f.Open()
        if err != nil {
            outFile.Close()
            return CheckResult{Success: false, Message: "Ошибка чтения файла из архива: " + err.Error()}
        }

        // Копируем данные из архива в файл на диске
        _, err = io.Copy(outFile, rc)
        outFile.Close() // Закрываем файл после записи
        rc.Close()      // Закрываем источник данных из архива

        if err != nil {
            return CheckResult{Success: false, Message: "Ошибка записи файла на диск: " + err.Error()}
        }
    }

    return CheckResult{
        Success: true,
        Message: "ServeHub-2 успешно скачан и развернут в рабочей папке!",
    }
}
```

**Умная проверка на фронтенде (main.js):**
```javascript
/**
 * Инициирует установку проекта (скачивание репозитория) и проверяет наличие 
 * файла конфигурации для определения дальнейшего пути пользователя.
 */
window.runInstallation = async function() {
    // 1. Получение ссылок на DOM-элементы для управления состоянием интерфейса
    const installText = document.getElementById('install-text');
    const spinner = document.getElementById('install-loading');
    const btnFinish = document.getElementById('btn-finish');
    const btnRetry = document.getElementById('btn-install-retry');
    const btnEditSecrets = document.getElementById('btn-edit-secrets');
    const btnStep6Back = document.getElementById('btn-step6-back');

    // 2. Настройка начального состояния UI: отображаем индикатор загрузки и блокируем кнопки
    spinner.style.display = 'block';
    installText.innerText = 'Загрузка архива проекта ServeHub-2 с GitHub в память...';
    installText.style.color = 'var(--text-secondary)';
    
    btnFinish.disabled = true;
    btnFinish.style.display = 'inline-block';
    btnRetry.style.display = 'none';
    btnEditSecrets.style.display = 'none';

    try {
        // 3. Вызов бэкенда для клонирования/скачивания проекта
        const result = await InstallProject();
        
        // 4. Обработка успешного скачивания
        if (result.success) {
            // Проверяем, был ли ранее создан файл secrets.yml
            const secretsCheck = await CheckSecrets();
            spinner.style.display = 'none';

            if (secretsCheck.success) {
                // Если конфигурация найдена: пропускаем настройку
                installText.innerHTML = `✓ ${result.message}<br><br><span style="color: var(--accent-soft); font-weight: bold;">ℹ Конфигурация найдена:</span> Файл <code style="background: var(--bg-3); padding: 2px 6px; border-radius: 4px;">secrets.yml</code> уже существует в проекте. Шаги создания и настройки будут пропущены.`;
                installText.style.color = 'var(--accent-bright)';
                
                btnFinish.disabled = false;
                btnFinish.innerText = 'Перейти к выбору развертки';
                btnFinish.setAttribute('onclick', 'goToStep(6)'); // Направляем на выбор сценария
                btnEditSecrets.style.display = 'inline-block';

                if (btnStep6Back) {
                    btnStep6Back.setAttribute('onclick', 'goToStep(3)');
                }
            } else {
                // Если конфигурации нет: направляем пользователя на шаг создания настроек
                installText.innerText = `✓ ${result.message}`;
                installText.style.color = 'var(--accent-bright)';
                
                btnFinish.disabled = false;
                btnFinish.innerText = 'Продолжить к настройке';
                btnFinish.setAttribute('onclick', 'goToStep(4)'); // Направляем на ввод секретов

                if (btnStep6Back) {
                    btnStep6Back.setAttribute('onclick', 'goToStep(5)');
                }
            }
        } else {
            // Ошибка при установке (например, нет интернета)
            spinner.style.display = 'none';
            installText.innerText = `✗ Ошибка установки: ${result.message}`;
            installText.style.color = 'var(--danger)';
            btnFinish.style.display = 'none';
            btnRetry.style.display = 'inline-block'; // Даем возможность повторить
        }
    } catch (err) {
        // 5. Обработка системных сбоев (ошибки API Wails или падение бэкенда)
        spinner.style.display = 'none';
        installText.innerText = 'Критическая ошибка в процессе установки:\n' + err;
        installText.style.color = 'var(--danger)';
        btnFinish.style.display = 'none';
        btnRetry.style.display = 'inline-block';
    }
}
```

### Шаг 4: Настройка secrets.yml

В полной версии представлен весь маппинг полей для парсинга конфигурации из YAML-файла. Парсер корректно собирает многострочные литералы (приватные SSH-ключи), отбрасывая лишние отступы.

**Заполнение формы из YAML (main.js):**
```javascript
/**
 * Словарь сопоставления ключей YAML (из secrets.yml) 
 * с ID элементов ввода в HTML-форме.
 * Позволяет легко поддерживать соответствие между структурой конфига и UI.
 */
const SECRETS_FIELD_MAP = {
    vps_public_ip: 'sec_vps_ip',
    vps_user: 'sec_vps_user',
    vps_root_password: 'sec_vps_root_pass',
    local_private_ip: 'sec_local_ip',
    local_user: 'sec_local_user',
    local_root_password: 'sec_local_root_pass',
    server_name: 'sec_server_name',
    admin_user: 'sec_admin_user',
    admin_password: 'sec_admin_pass',
    email: 'sec_email',
    telegram_token: 'sec_tg_token',
    telegram_chat_id: 'sec_tg_chat_id',
    webnames_apikey: 'sec_webnames',
    postgres_db_nextcloud: 'sec_pg_nc',
    postgres_db_vaultwarden: 'sec_pg_vw',
    postgres_user: 'sec_pg_user',
    postgres_password: 'sec_pg_pass',
    nextcloud_redis_pass: 'sec_redis_pass',
    secret_vaultwarden_password: 'sec_vw_admin_pass',
    ssh_public_key: 'sec_public_ssh_key',
    ssh_private_key: 'sec_private_ssh_key',
    borgmatic_encryption_passphrase: 'sec_borg_pass',
    backup_disk_uuid: 'sec_disk_uuid',
};

/**
 * Парсит содержимое YAML-файла и заполняет соответствующие поля HTML-формы.
 * Поддерживает как простые строковые значения, так и многострочные литералы (для SSH-ключей).
 * @param {string} yamlText - Исходный текст файла secrets.yml.
 */
function populateSecretsForm(yamlText) {
    const lines = yamlText.split('\n');
    let i = 0;

    while (i < lines.length) {
        const line = lines[i];

        // 1. Обработка многострочных литералов (ключ вида `key: |`)
        // Используется для SSH-ключей, которые могут содержать много строк
        const literalMatch = line.match(/^([a-z_]+):\s*\|\s*$/);
        if (literalMatch) {
            const fieldId = SECRETS_FIELD_MAP[literalMatch[1]];
            i++; // Переходим к следующей строке (содержимое блока)
            const blockLines = [];
            
            // Читаем все строки, которые имеют отступ (4 пробела)
            while (i < lines.length && (lines[i].startsWith('    ') || lines[i].trim() === '')) {
                // Удаляем 4 ведущих пробела, чтобы восстановить оригинальное содержимое
                blockLines.push(lines[i].replace(/^ {4}/, ''));
                i++;
            }
            
            // Удаляем пустые строки в конце блока
            while (blockLines.length && blockLines[blockLines.length - 1] === '') {
                blockLines.pop();
            }
            
            // Записываем собранный блок в соответствующее поле формы
            if (fieldId) {
                const el = document.getElementById(fieldId);
                if (el) el.value = blockLines.join('\n');
            }
            continue; // Пропускаем инкремент i в конце цикла, так как мы уже продвинулись
        }

        // 2. Обработка стандартных строковых переменных (ключ вида `key: "value"`)
        const simpleMatch = line.match(/^([a-z_]+):\s*"(.*)"\s*$/);
        if (simpleMatch) {
            const fieldId = SECRETS_FIELD_MAP[simpleMatch[1]];
            if (fieldId) {
                const el = document.getElementById(fieldId);
                // Записываем значение из группы регулярного выражения во второе поле ввода
                if (el) el.value = simpleMatch[2];
            }
        }
        i++;
    }
}
```

### Шаг 5: Проверка конфигурации

При нажатии на “Продолжить” мы собираем значения из 23 полей формы и генерируем единый шаблон конфигурации.

**Генерация шаблона (main.js):**
```javascript
/**
 * Функция собирает данные из формы конфигурации, формирует YAML-файл в виде строки
 * и подготавливает его для предварительного просмотра пользователем.
 */
window.generateAndReviewYaml = function() {
    // 1. Сбор данных из всех полей ввода формы по их ID
    const vpsIp = document.getElementById('sec_vps_ip').value;
    const vpsUser = document.getElementById('sec_vps_user').value;
    const vpsRootPass = document.getElementById('sec_vps_root_pass').value;
    const localIp = document.getElementById('sec_local_ip').value;
    const localUser = document.getElementById('sec_local_user').value;
    const localRootPass = document.getElementById('sec_local_root_pass').value;
    
    const serverName = document.getElementById('sec_server_name').value;
    const adminUser = document.getElementById('sec_admin_user').value;
    const adminPass = document.getElementById('sec_admin_pass').value;
    const email = document.getElementById('sec_email').value;
    
    const tgToken = document.getElementById('sec_tg_token').value;
    const tgChatId = document.getElementById('sec_tg_chat_id').value;
    const webnamesKey = document.getElementById('sec_webnames').value;
    
    const pgNc = document.getElementById('sec_pg_nc').value;
    const pgVw = document.getElementById('sec_pg_vw').value;
    const pgUser = document.getElementById('sec_pg_user').value;
    const pgPass = document.getElementById('sec_pg_pass').value;
    const redisPass = document.getElementById('sec_redis_pass').value;
    
    const vwAdminPass = document.getElementById('sec_vw_admin_pass').value;
    const publicsshKey = document.getElementById('sec_public_ssh_key').value;
    const privatesshKey = document.getElementById('sec_private_ssh_key').value;
    const borgPass = document.getElementById('sec_borg_pass').value;
    const diskUuid = document.getElementById('sec_disk_uuid').value;

    // 2. Форматирование приватного SSH ключа:
    // YAML требует специфического отступа для многострочных литералов (символ `|`).
    // Мы разбиваем строку по символу переноса и добавляем 4 пробела к каждой строке.
    const formattedPrivateKey = privatesshKey
        .split('\n')
        .map(line => '    ' + line)
        .join('\n');

    // 3. Формирование YAML-документа:
    // Используем шаблонные строки (` `) для вставки переменных. 
    // Это обеспечивает правильную структуру конфига, понятную для Ansible.
    generatedYamlString = `# ==============================================================================
#                 ServeHub-2: СГЕНЕРИРОВАННЫЙ КОНФИГУРАЦИОННЫЙ SECRETS.YML
# ==============================================================================

# 1. СЕТЕВАЯ ИНФРАСТРУКТУРА И ПОЛЬЗОВАТЕЛИ
vps_public_ip: "${vpsIp}"
vps_user: "${vpsUser}"
vps_root_password: "${vpsRootPass}"
local_private_ip: "${localIp}"
local_user: "${localUser}"
local_root_password: "${localRootPass}"

# 2. ГЛОБАЛЬНЫЕ НАСТРОЙКИ
server_name: "${serverName}"
admin_user: "${adminUser}"
admin_password: "${adminPass}"
email: "${email}"

# 3. НАСТРОЙКИ АЛЕРТИНГА
telegram_token: "${tgToken}"
telegram_chat_id: "${tgChatId}"

# 4. ИНТЕГРАЦИЯ DNS
webnames_apikey: "${webnamesKey}"

# 5. НАСТРОЙКИ POSTGRESQL
postgres_db_nextcloud: "${pgNc}"
postgres_db_vaultwarden: "${pgVw}"
postgres_user: "${pgUser}"
postgres_password: "${pgPass}"
nextcloud_redis_pass: "${redisPass}"

# 6. СЕКРЕТЫ СЕРВИСОВ
secret_vaultwarden_password: "${vwAdminPass}"
ssh_public_key: "${publicsshKey}"
ssh_private_key: |
${formattedPrivateKey}
borgmatic_encryption_passphrase: "${borgPass}"
backup_disk_uuid: "${diskUuid}"
`;

    // 4. Отображение результата:
    // Выводим полученный YAML в блок предпросмотра на странице
    document.getElementById('yaml-preview').innerText = generatedYamlString;

    // 5. Управление навигацией:
    // Настраиваем кнопку "Назад", чтобы она вела на экран редактирования данных
    const btnStep6Back = document.getElementById('btn-step6-back');
    if (btnStep6Back) {
        btnStep6Back.setAttribute('onclick', 'goToStep(5)');
    }
    
    // Переход к шагу 5 (просмотр и подтверждение)
    goToStep(5);
}
```

**Сохранение файла (app.go):**
```go
// SaveSecrets сохраняет сгенерированный YAML-контент в файл конфигурации проекта.
// Метод принимает строку с содержимым будущего secrets.yml и возвращает результат операции.
func (a *App) SaveSecrets(yamlContent string) CheckResult {
    // 1. Определение пути к файлу:
    // Формируем относительный путь к директории, где Ansible ожидает увидеть файл переменных.
    // Используется filepath.Join для кроссплатформенной совместимости путей.
    targetDir := filepath.Join(".", "ServeHub-2-main", "ansible", "vars")

    // 2. Подготовка директорий:
    // os.MkdirAll создает всю цепочку директорий, если они отсутствуют.
    // os.ModePerm (0777) устанавливает полные права доступа для создаваемых папок.
    err := os.MkdirAll(targetDir, os.ModePerm)
    if err != nil {
        // Возвращаем ошибку, если у приложения нет прав на создание папок или возникла проблема с диском.
        return CheckResult{
            Success: false, 
            Message: "Не удалось создать директорию конфигурации: " + err.Error(),
        }
    }

    // 3. Формирование пути к файлу:
    targetFile := filepath.Join(targetDir, "secrets.yml")

    // 4. Запись данных:
    // os.WriteFile создает файл (или перезаписывает существующий) и записывает в него байты.
    // Права 0644 (rw-r--r--) позволяют владельцу читать и писать в файл, а остальным — только читать.
    // Это стандартный уровень безопасности для конфигурационных файлов с секретами.
    err = os.WriteFile(targetFile, []byte(yamlContent), 0644)
    if err != nil {
        // Если запись не удалась (например, файл занят или доступ ограничен), возвращаем ошибку.
        return CheckResult{
            Success: false, 
            Message: "Не удалось записать файл secrets.yml: " + err.Error(),
        }
    }

    // 5. Успешное завершение:
    // Конфигурация успешно сохранена на диск.
    return CheckResult{
        Success: true, 
        Message: "Файл secrets.yml успешно создан!",
    }
}
```

### Шаг 6: Выбор сценария деплоя

Здесь реализована логика выделения выбранной карточки развертывания и подтверждения старта.

**Выбор карточки (main.js):**
```javascript
// Глобальная переменная для хранения выбранного пользователем ID сценария развертывания.
// Инициализируется нулем, так как выбор еще не сделан.
let selectedOptionNum = 0; 

/**
 * Обрабатывает выбор карточки сценария деплоя пользователем.
 * @param {number} num - Идентификатор выбранного сценария.
 */
window.selectDeployOption = function(num) {
    selectedOptionNum = num;

    // 1. Сброс визуального состояния:
    // Находим все карточки с классом 'deploy-option-card' и убираем у них класс 'selected',
    // чтобы при выборе новой карточки старая потеряла выделение.
    document.querySelectorAll('.deploy-option-card').forEach(card => {
        card.classList.remove('selected');
    });

    // 2. Установка нового выделения:
    // Находим карточку по ID (например, 'opt-1') и добавляем ей класс 'selected' для подсветки через CSS.
    const selectedCard = document.getElementById(`opt-${num}`);
    if (selectedCard) {
        selectedCard.classList.add('selected');
    }

    // 3. Активация кнопки перехода:
    // Делаем кнопку 'Далее' доступной, так как сценарий теперь выбран.
    const btnNext = document.getElementById('btn-deploy-next');
    if (btnNext) {
        btnNext.disabled = false;
    }
}

/**
 * Подготавливает данные для финального экрана перед началом выполнения Ansible-сценария.
 */
window.confirmDeployment = function() {
    // Если пользователь не выбрал сценарий (выбран 0), прерываем выполнение.
    if (selectedOptionNum === 0) return;

    // Словарь текстовых описаний для понятного отображения выбранного пункта в UI.
    const optionTexts = {
        1: "Пункт 1. Полный деплой (Локальный + VPS)",
        2: "Пункт 2. Запуск сервисов (Локальный + VPS)",
        3: "Пункт 3. Полный деплой (Локальный)",
        4: "Пункт 4. Запуск сервисов (Локальный)",
        5: "Пункт 5. Полный деплой (VPS)",
        6: "Пункт 6. Запуск сервисов (VPS)"
    };

    // Обновляем текстовое поле на экране подтверждения, подставляя описание выбранного сценария.
    const confirmTextEl = document.getElementById('confirm-option-text');
    if (confirmTextEl) {
        confirmTextEl.innerText = optionTexts[selectedOptionNum] || `Сценарий №${selectedOptionNum}`;
    }

    // 4. Проверка состояния отладки (логирование):
    // Проверяем, включен ли переключатель "Детализация логов" (-vvv).
    const isVerbose = document.getElementById('verbose-logs-toggle').checked;
    const confirmVerboseEl = document.getElementById('confirm-verbose-text');
    
    // Выводим статус отладки на экран подтверждения для информирования пользователя.
    if (confirmVerboseEl) {
        confirmVerboseEl.innerText = isVerbose ? "Да (-vvv)" : "Нет";
    }

    // Переход к шагу 7 — непосредственно экрану запуска деплоя.
    goToStep(7);
}
```

### Шаг 7: Запуск деплоя

Это самый сложный шаг, где происходит запуск процесса в псевдотерминале и трансляция логов в реальном времени. В полных функциях видно, как обрабатываются все 6 вариантов деплоя через `switch/case`, а также как удаляются цветовые коды из потока логов.

**Бэкенд: Запуск Ansible в PTY (app.go):**
```go
// ansiEscape — это регулярное выражение для поиска и удаления ANSI escape-последовательностей.
// Эти коды используются терминалом для раскрашивания текста (цвета шрифта, фона, жирность).
// Поскольку мы выводим логи в HTML-блок <pre>, нам нужно их очистить, чтобы в логах не появлялись "мусорные" символы.
var ansiEscape = regexp.MustCompile(
	"\x1b\\][^\x07\x1b]*(\x07|\x1b\\\\)" + // Коды управления операционной системой
		"|\x1b\\[[0-9;?]*[a-zA-Z]" + // Коды форматирования графики (цвета и стили)
		"|\x1b[()[A-Za-z0-9]" + // Коды выбора кодировки и наборов символов
		"|\x1b[=>78]", // Коды инициализации/сброса терминала
)

// stripAnsi удаляет все найденные ANSI-коды из строки.
func stripAnsi(s string) string {
	return ansiEscape.ReplaceAllString(s, "")
}

// RunDeployment выполняет Ansible-плэйбуки через Docker-контейнер, 
// используя PTY для эмуляции интерактивного терминала.
func (a *App) RunDeployment(option int, verbose bool) CheckResult {
	var steps []string
	debugArgs := ""
	if verbose {
		debugArgs = " -vvv" // Добавляем флаг подробного логирования, если нужно
	}

	// 1. Формирование сценария деплоя на основе выбора пользователя
	// Каждый шаг — это отдельный запуск команды ansible-playbook через docker compose
	switch option {
	case 1:
		steps = []string{
			fmt.Sprintf("ansible-playbook -i ansible/inventory.ini ansible/deploy.yml --limit vps,local --tags bootstrap%s", debugArgs),
			fmt.Sprintf("ansible-playbook -i ansible/inventory.ini ansible/deploy.yml --limit vps --skip-tags bootstrap%s", debugArgs),
			fmt.Sprintf("ansible-playbook -i ansible/inventory.ini ansible/deploy.yml --limit local --skip-tags bootstrap%s", debugArgs),
		}
	// ... (другие кейсы)
	default:
		return CheckResult{Success: false, Message: "Неизвестный сценарий развертывания!"}
	}

	// 2. Последовательное выполнение команд из списка steps
	for _, ansibleCmd := range steps {
		wailsRuntime.EventsEmit(a.ctx, "deploy-log")

		// Команда для исполнения внутри контейнера
		cmdArgs := []string{
			"compose",
			"-f", "./ServeHub-2-main/docker-compose.ansible.yaml",
			"run", "--rm", "ansible",
			"sh", "-c", ansibleCmd,
		}

		// 3. Создание псевдотерминала (PTY):
		// Ansible требует PTY, чтобы корректно отображать прогресс-бары и цветовую разметку.
		ptty, err := pty.New()
		if err != nil {
			return CheckResult{Success: false, Message: fmt.Sprintf("Не удалось создать псевдотерминал: %s", err.Error())}
		}
		_ = ptty.Resize(200, 50) // Устанавливаем размер окна терминала для корректного переноса строк

		// Инициализация процесса
		cmd := ptty.CommandContext(a.ctx, "docker", cmdArgs...)
		cmd.Env = append(os.Environ(), "TERM=xterm-256color") // Эмулируем полноценный терминал

		if err := cmd.Start(); err != nil {
			ptty.Close()
			return CheckResult{Success: false, Message: fmt.Sprintf("Не удалось запустить Docker: %s", err.Error())}
		}

		// Сохраняем ссылки в структуре App, чтобы функция SendEnter могла ими воспользоваться
		a.ptty = ptty
		a.cmdStdin = ptty
		a.activeCmd = cmd

		// 4. Асинхронное чтение логов:
		// Создаем отдельную горутину, которая постоянно читает вывод из PTY
		go func(p pty.Pty) {
			scanner := bufio.NewScanner(p)
			scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024) // Увеличиваем буфер для длинных строк лога
			for scanner.Scan() {
				clean := stripAnsi(scanner.Text())
				if clean == "" {
					continue
				}
				// Отправляем строку на фронтенд (Wails событие)
				wailsRuntime.EventsEmit(a.ctx, "deploy-log", clean+"\n")
			}
		}(ptty)

		// Ждем завершения текущего плэйбука
		waitErr := cmd.Wait()

		// Очистка ресурсов после завершения команды
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
```

**Фронтенд: Прослушивание логов (main.js):**
```javascript
/**
 * Основная функция для инициализации и запуска процесса деплоя через Ansible.
 * Управляет состоянием интерфейса, подпиской на потоки логов и обработкой ошибок.
 */
window.startDeployment = async function() {
    // 1. Получение ссылок на DOM-элементы для управления интерфейсом
    const logContainer = document.getElementById('deploy-logs-container');
    const logPre = document.getElementById('deploy-logs');
    const btnRun = document.getElementById('btn-deploy-run');
    const btnBack = document.getElementById('btn-deploy-back');
    const btnExit = document.getElementById('btn-deploy-exit');
    const btnEnter = document.getElementById('btn-deploy-enter');
    const isVerbose = document.getElementById('verbose-logs-toggle').checked;

    // 2. Первичная настройка UI: отображение контейнера логов и статусная строка
    logContainer.style.display = 'block';
    logPre.innerText = 'Подготовка окружения и инициализация процесса деплоя...\n';

    // Блокировка кнопок навигации, чтобы пользователь не мог прервать процесс
    btnRun.disabled = true;
    btnBack.disabled = true;
    if (btnExit) btnExit.style.display = 'none';
    // Показываем кнопку "Enter" для интерактивного взаимодействия с Ansible
    if (btnEnter) btnEnter.style.display = 'block';

    const postDeployGuide = document.getElementById('post-deploy-guide');
    if (postDeployGuide) postDeployGuide.style.display = 'none';

    /**
     * Вспомогательная функция для добавления логов в терминал с автоскроллом.
     */
    function addLog(message) {
        const logPre = document.getElementById('deploy-logs');
        // Проверяем, находится ли скролл в самом низу (погрешность 5px)
        const isAtBottom = (logPre.scrollHeight - logPre.clientHeight) <= (logPre.scrollTop + 5);
        
        // Добавляем текст в блок логов
        logPre.innerText += message;
        
        // Если пользователь не прокручивал лог вручную вверх, автоматически прокручиваем вниз
        if (isAtBottom) {
            logPre.scrollTop = logPre.scrollHeight;
        }
    }

    // 3. Подписка на событие логов:
    // Мы используем EventsOn из Wails, чтобы слушать события, приходящие из Go.
    // Флаг deployLogListened предотвращает многократную подписку при повторных запусках.
    if (!window.deployLogListened) {
        EventsOn('deploy-log', (message) => {
            addLog(message);
        });
        window.deployLogListened = true;
    }

    // 4. Запуск основного процесса деплоя:
    try {
        // Вызов функции Go-бэкенда. Выполнение блокирует этот поток, пока процесс не завершится.
        const result = await RunDeployment(selectedOptionNum, isVerbose);
        
        // Скрываем кнопку ввода, так как деплой завершился
        if (btnEnter) btnEnter.style.display = 'none';

        // 5. Обработка результата (Успех/Ошибка Ansible):
        if (result.success) {
            logPre.innerText += `\n[ УСПЕХ ]: ${result.message}\n`;
            logPre.style.borderColor = '#818cf8'; // Подсветка границ в случае успеха
            btnRun.style.display = 'none';
            btnBack.style.display = 'none';
            if (btnExit) {
                btnExit.style.display = 'block';
                btnExit.className = 'btn-primary';
            }

            // Показываем руководство, что делать после успешного деплоя
            const guide = document.getElementById('post-deploy-guide');
            if (guide) guide.style.display = 'block';
        } else {
            // Обработка логических ошибок Ansible (например, ошибка конфигурации)
            logPre.innerText += `\n[ ОШИБКА ]: Деплой завершился неудачей.\nПричина: ${result.message}\n`;
            logPre.style.borderColor = '#e5484d'; // Подсветка границ (красный)
            btnRun.disabled = false;
            btnBack.disabled = false;
            if (btnExit) btnExit.style.display = 'block';
        }
    } catch (err) {
        // 6. Обработка критических системных ошибок (например, падение самого Docker/Ansible)
        if (btnEnter) btnEnter.style.display = 'none';
        logPre.innerText += `\n[ КРИТИЧЕСКАЯ ОШИБКА СИСТЕМЫ ]: Ошибка вызова бэкенда:\n${err}\n`;
        logPre.style.borderColor = '#e5484d';

        btnRun.disabled = false;
        btnBack.disabled = false;
        if (btnExit) btnExit.style.display = 'block';
    }
}
```

**Отправка сигнала Enter в процесс (app.go):**
```go
// SendEnter отправляет сигнал нажатия клавиши "Enter" в интерактивный процесс Ansible.
// Данный метод используется, когда Ansible приостанавливает выполнение для ожидания
// пользовательского ввода (например, подтверждение sudo-пароля или перезаписи файла).
func (a *App) SendEnter() CheckResult {
    // 1. Проверка состояния процесса:
    // Мы проверяем, инициализирован ли интерфейс записи в стандартный поток ввода (stdin).
    // Если cmdStdin равен nil, значит процесс либо еще не запущен, либо уже завершился,
    // и отправка сигнала невозможна.
    if a.cmdStdin == nil {
        return CheckResult{
            Success: false, 
            Message: "Процесс деплоя не запущен или не ожидает ввода",
        }
    }

    // 2. Отправка управляющей последовательности:
    // Мы записываем в поток ввода "\r\n" (Return + Newline). 
    // Это стандартный способ эмуляции нажатия клавиши Enter для POSIX-совместимых терминалов,
    // через которые Ansible взаимодействует с пользователем.
    _, err := a.cmdStdin.Write([]byte("\r\n"))
    
    // 3. Обработка ошибок записи:
    // Если возникла ошибка записи в поток (например, канал был закрыт до того, как
    // мы успели отправить байты), возвращаем статус ошибки вместе с описанием причины.
    if err != nil {
        return CheckResult{
            Success: false, 
            Message: fmt.Sprintf("Не удалось отправить Enter: %s", err.Error()),
        }
    }

    // 4. Подтверждение операции:
    // Если запись прошла успешно, возвращаем статус успеха.
    return CheckResult{
        Success: true, 
        Message: "Сигнал Enter успешно отправлен",
    }
}
```

Для фронтенда также реализована функция вызова бэкенда:
```javascript
/**
 * Функция-обертка для отправки сигнала Enter в запущенный процесс Ansible.
 * Используется в интерфейсе для взаимодействия с интерактивными запросами Ansible
 * (например, подтверждение пароля или перезаписи файлов), если они возникают.
 */
window.sendEnterToAnsible = async function() {
    try {
        // Вызываем GO-функцию SendEnter, которая реализована в app.go
        // Она обращается напрямую к стандартному вводу (stdin) запущенного псевдотерминала (PTY)
        const result = await SendEnter();
        
        // Проверяем результат выполнения операции на бэкенде
        if (!result.success) {
            // Если возникла логическая ошибка (например, процесс уже завершился), 
            // выводим предупреждение в консоль разработчика, не прерывая работу приложения
            console.warn("Попытка отправки Enter была отклонена:", result.message);
        } else {
            // Опционально: можно добавить лог для отладки успешного действия
            console.log("Сигнал Enter успешно отправлен в процесс.");
        }
    } catch (err) {
        // Обработка критических ошибок (например, потеря связи с бэкендом Wails)
        // Выводим детальную информацию об ошибке в консоль
        console.error("Произошла системная ошибка при попытке отправки Enter:", err);
    }
}
```

## Заключение

Постарался подробно пройти по основным моментам установщика Go Wails. Некоторые моменты пришлось опустить, однако в целом полная архитектура и работа наглядно видны, надеюсь что это так.

Вероятно это последняя статья по проекту ServeHub-2, я еще конечно могу рассказать про сборку AppImage, более подробно пройтись по функциям и тд, но вероятно я это сделаю если будет еще что-то, что можно добавить для полноценной статьи.

Внешний вид установщика можно посмотреть, скачав бинарник или AppImage из релизов проекта, я честно пытался встроить нормальное видео, но у меня не очень получилось, поэтому вот так.