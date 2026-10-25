import './style.css';
import { CheckDocker, InstallProject, SaveSecrets, CloseApp, MinimizeApp, MaximizeApp, RunDeployment, CheckSecrets, LoadSecrets, SendEnter } from '../wailsjs/go/main/App.js';
import { EventsOn } from '../wailsjs/runtime/runtime.js';

let generatedYamlString = "";

function yamlQuote(value) {
    return JSON.stringify(String(value ?? ""));
}

const STEP_TITLES = {
    1: 'Добро пожаловать',
    2: 'Проверка требований',
    3: 'Установка компонентов',
    4: 'Настройка secrets.yml',
    5: 'Проверка конфигурации',
    6: 'Выбор сценария деплоя',
    7: 'Запуск деплоя',
};

function updateStepProgress(stepNumber) {
    const totalSteps = 7;
    const fill = document.getElementById('step-progress-fill');
    const currentEl = document.getElementById('step-progress-current');
    const titleEl = document.getElementById('step-progress-title');

    if (fill) {
        const pct = ((stepNumber - 1) / (totalSteps - 1)) * 100;
        fill.style.width = `${pct}%`;
    }
    if (currentEl) currentEl.innerText = stepNumber;
    if (titleEl) titleEl.innerText = STEP_TITLES[stepNumber] || '';

    document.querySelectorAll('.step-dot').forEach((dot) => {
        const dotStep = parseInt(dot.getAttribute('data-step'), 10);
        dot.classList.remove('completed', 'active');
        if (dotStep < stepNumber) dot.classList.add('completed');
        if (dotStep === stepNumber) dot.classList.add('active');
    });
}

window.goToStep = function(stepNumber) {
    document.querySelectorAll('.step').forEach(step => {
        step.classList.remove('active');
    });

    const nextStep = document.getElementById(`step-${stepNumber}`);
    if (nextStep) {
        nextStep.classList.add('active');
    }

    updateStepProgress(stepNumber);

    if (stepNumber === 2) {
        runDockerCheck();
    }

    if (stepNumber === 3) {
        runInstallation();
    }
}

window.runDockerCheck = async function() {
    const statusText = document.getElementById('status-text');
    const spinner = document.getElementById('status-loading');
    const btnNext = document.getElementById('btn-step2-next');
    const btnRetry = document.getElementById('btn-retry');

    spinner.style.display = 'block';
    spinner.className = 'spinner';
    statusText.innerText = 'Проверяем Docker и Docker Compose...';
    statusText.style.color = '#9a9aa0';
    btnNext.disabled = true;
    btnRetry.style.display = 'none';

    await new Promise(resolve => setTimeout(resolve, 700));

    try {
        const result = await CheckDocker();

        spinner.style.display = 'none';

        if (result.success) {
            statusText.innerText = `✓ ${result.message}`;
            statusText.style.color = '#818cf8';
            btnNext.disabled = false;
        } else {
            statusText.innerText = `✗ Ошибка: ${result.message}.`;
            statusText.style.color = '#e5484d';
            btnRetry.style.display = 'block';
        }
    } catch (err) {
        spinner.style.display = 'none';
        statusText.innerText = 'Критическая ошибка при вызове проверки бэкенда:\n' + err;
        statusText.style.color = '#e5484d';
        btnRetry.style.display = 'block';
    }
}

window.runInstallation = async function() {
    const installText = document.getElementById('install-text');
    const spinner = document.getElementById('install-loading');
    const btnFinish = document.getElementById('btn-finish');
    const btnRetry = document.getElementById('btn-install-retry');
    const btnEditSecrets = document.getElementById('btn-edit-secrets');
    const btnStep6Back = document.getElementById('btn-step6-back');

    spinner.style.display = 'block';
    installText.innerText = 'Загрузка архива проекта ServeHub-2 с GitHub в память...';
    installText.style.color = 'var(--text-secondary)';
    btnFinish.disabled = true;
    btnFinish.style.display = 'inline-block';
    btnRetry.style.display = 'none';
    btnEditSecrets.style.display = 'none';

    try {
        const result = await InstallProject();
        
        if (result.success) {
            const secretsCheck = await CheckSecrets();
            
            spinner.style.display = 'none';

            if (secretsCheck.success) {
                const existingSecrets = await LoadSecrets();
                const readHermesSecret = (key) => {
                    if (!existingSecrets.success) return '';
                    const line = existingSecrets.content.split(/\r?\n/).find(item => item.startsWith(key + ':'));
                    if (!line) return '';
                    return line.slice(line.indexOf(':') + 1).trim().replace(/^["']|["']$/g, '');
                };
                const currentHermesBaseUrl = readHermesSecret('hermes_api_base_url');
                const currentHermesApiKey = readHermesSecret('hermes_api_key');
                const currentHermesModel = readHermesSecret('hermes_model');
                const currentHermesDashboardSecret = readHermesSecret('hermes_dashboard_basic_auth_secret');
                const hasHermesSettings = existingSecrets.success &&
                    (currentHermesBaseUrl.startsWith('https://') || currentHermesBaseUrl.startsWith('http://')) &&
                    Boolean(currentHermesApiKey) &&
                    !currentHermesApiKey.toLowerCase().startsWith('replace_with_') &&
                    Boolean(currentHermesModel) &&
                    /^[a-f0-9]{64}$/i.test(currentHermesDashboardSecret);

                spinner.style.display = 'none';

                if (!hasHermesSettings && existingSecrets.success) {
                    populateSecretsForm(existingSecrets.content);
                    installText.innerText = 'Обнаружена существующая конфигурация. Для автоматического развёртывания обязательного Hermes Agent заполните API-ключ и проверьте Base URL и модель. Остальные сохранённые настройки загружены из secrets.yml.';
                    installText.style.color = 'var(--accent-bright)';
                    btnFinish.disabled = false;
                    btnFinish.innerText = 'Настроить Hermes';
                    btnFinish.setAttribute('onclick', 'goToStep(4)');
                    btnEditSecrets.style.display = 'none';
                    if (btnStep6Back) {
                        btnStep6Back.setAttribute('onclick', 'goToStep(3)');
                    }
                    return;
                }

                installText.innerHTML = `✓ ${result.message}<br><br><span style="color: var(--accent-soft); font-weight: bold;">ℹ Конфигурация найдена:</span> Файл <code style="background: var(--bg-3); padding: 2px 6px; border-radius: 4px;">secrets.yml</code> уже существует в проекте. Шаги создания и настройки будут пропущены.`;
                installText.style.color = 'var(--accent-bright)';
                
                btnFinish.disabled = false;
                btnFinish.innerText = 'Перейти к выбору развертки';
                btnFinish.setAttribute('onclick', 'goToStep(6)');

                btnEditSecrets.style.display = 'inline-block';

                if (btnStep6Back) {
                    btnStep6Back.setAttribute('onclick', 'goToStep(3)');
                }

            } else {
                installText.innerText = `✓ ${result.message}`;
                installText.style.color = 'var(--accent-bright)';
                
                btnFinish.disabled = false;
                btnFinish.innerText = 'Продолжить к настройке';
                btnFinish.setAttribute('onclick', 'goToStep(4)');

                if (btnStep6Back) {
                    btnStep6Back.setAttribute('onclick', 'goToStep(5)');
                }
            }
        } else {
            spinner.style.display = 'none';
            installText.innerText = `✗ Ошибка установки: ${result.message}`;
            installText.style.color = 'var(--danger)';
            btnFinish.style.display = 'none';
            btnRetry.style.display = 'inline-block';
        }
    } catch (err) {
        spinner.style.display = 'none';
        installText.innerText = 'Критическая ошибка в процессе установки:\n' + err;
        installText.style.color = 'var(--danger)';
        btnFinish.style.display = 'none';
        btnRetry.style.display = 'inline-block';
    }
}

const SECRETS_FIELD_MAP = {
    vps_public_ip: 'sec_vps_ip',
    vps_user: 'sec_vps_user',
    vps_root_password: 'sec_vps_root_pass',
    local_private_ip: 'sec_local_ip',
    local_user: 'sec_local_user',
    local_root_password: 'sec_local_root_pass',
    server_name: 'sec_server_name',
    admin_user: 'sec_admin_user',
    hbox_pepper: 'sec_hbox_pepper',
    admin_password: 'sec_admin_pass',
    email: 'sec_email',
    telegram_token: 'sec_tg_token',
    telegram_chat_id: 'sec_tg_chat_id',
    webnames_apikey: 'sec_webnames',
    postgres_user: 'sec_pg_user',
    postgres_password: 'sec_pg_pass',
    nextcloud_redis_pass: 'sec_redis_pass',
    secret_vaultwarden_password: 'sec_vw_admin_pass',
    ssh_public_key: 'sec_public_ssh_key',
    ssh_private_key: 'sec_private_ssh_key',
    borgmatic_encryption_passphrase: 'sec_borg_pass',
    backup_disk_uuid: 'sec_disk_uuid',
    offsite_s3_endpoint: 'sec_s3_endpoint',
    offsite_s3_region: 'sec_s3_region',
    offsite_s3_bucket: 'sec_s3_bucket',
    offsite_s3_access_key_id: 'sec_s3_access_key',
    offsite_s3_secret_access_key: 'sec_s3_secret_key',
    offsite_s3_workload_prefix: 'sec_s3_workload_prefix',
    offsite_s3_edge_prefix: 'sec_s3_edge_prefix',
    hermes_api_base_url: 'sec_hermes_api_base_url',
    hermes_api_key: 'sec_hermes_api_key',
    hermes_model: 'sec_hermes_model',
    hermes_dashboard_basic_auth_secret: 'sec_hermes_dashboard_basic_auth_secret',
};

function populateSecretsForm(yamlText) {
    const lines = yamlText.split('\n');
    let i = 0;

    while (i < lines.length) {
        const line = lines[i];

        const literalMatch = line.match(/^([a-z_]+):\s*\|\s*$/);
        if (literalMatch) {
            const fieldId = SECRETS_FIELD_MAP[literalMatch[1]];
            i++;
            const blockLines = [];
            while (i < lines.length && (lines[i].startsWith('    ') || lines[i].trim() === '')) {
                blockLines.push(lines[i].replace(/^ {4}/, ''));
                i++;
            }
            while (blockLines.length && blockLines[blockLines.length - 1] === '') {
                blockLines.pop();
            }
            if (fieldId) {
                const el = document.getElementById(fieldId);
                if (el) el.value = blockLines.join('\n');
            }
            continue;
        }

        const simpleMatch = line.match(/^([a-z_]+):\s*"(.*)"\s*$/);
        if (simpleMatch) {
            const fieldId = SECRETS_FIELD_MAP[simpleMatch[1]];
            if (fieldId) {
                const el = document.getElementById(fieldId);
                if (el) {
                    try {
                        el.value = JSON.parse('"' + simpleMatch[2] + '"');
                    } catch {
                        el.value = simpleMatch[2];
                    }
                }
            }
        }

        i++;
    }
}

window.editExistingSecrets = async function() {
    try {
        const result = await LoadSecrets();
        if (result.success) {
            populateSecretsForm(result.content);
            goToStep(4);
        } else {
            alert(`Не удалось загрузить текущую конфигурацию: ${result.message}`);
        }
    } catch (err) {
        alert(`Критическая ошибка бэкенда при загрузке конфигурации: ${err}`);
    }
}

window.generateAndReviewYaml = function() {
    const vpsIp = document.getElementById('sec_vps_ip').value;
    const vpsUser = document.getElementById('sec_vps_user').value;
    const vpsRootPass = document.getElementById('sec_vps_root_pass').value;
    const localIp = document.getElementById('sec_local_ip').value;
    const localUser = document.getElementById('sec_local_user').value;
    const localRootPass = document.getElementById('sec_local_root_pass').value;
    const hboxPepper = document.getElementById('sec_hbox_pepper').value;
    
    const serverName = document.getElementById('sec_server_name').value;
    const adminUser = document.getElementById('sec_admin_user').value;
    const adminPass = document.getElementById('sec_admin_pass').value;
    const email = document.getElementById('sec_email').value;
    
    const tgToken = document.getElementById('sec_tg_token').value;
    const tgChatId = document.getElementById('sec_tg_chat_id').value;
    const webnamesKey = document.getElementById('sec_webnames').value;
    
    const pgUser = document.getElementById('sec_pg_user').value;
    const pgPass = document.getElementById('sec_pg_pass').value;
    const redisPass = document.getElementById('sec_redis_pass').value;
    
    const vwAdminPass = document.getElementById('sec_vw_admin_pass').value;
    const publicsshKey = document.getElementById('sec_public_ssh_key').value;
    const privatesshKey = document.getElementById('sec_private_ssh_key').value;
    const borgPass = document.getElementById('sec_borg_pass').value;
    const diskUuid = document.getElementById('sec_disk_uuid').value;
    const s3Endpoint = document.getElementById('sec_s3_endpoint').value.trim().replace(/\/+$/, '');
    const s3Region = document.getElementById('sec_s3_region').value.trim();
    const s3Bucket = document.getElementById('sec_s3_bucket').value.trim();
    const s3AccessKey = document.getElementById('sec_s3_access_key').value.trim();
    const s3SecretKey = document.getElementById('sec_s3_secret_key').value.trim();
    const s3WorkloadPrefix = document.getElementById('sec_s3_workload_prefix').value.trim();
    const s3EdgePrefix = document.getElementById('sec_s3_edge_prefix').value.trim();

    const hermesApiBaseUrl = document.getElementById('sec_hermes_api_base_url').value.trim().replace(/\/+$/, '');
    const hermesApiKey = document.getElementById('sec_hermes_api_key').value.trim();
    const hermesModel = document.getElementById('sec_hermes_model').value.trim();
    const hermesSecretField = document.getElementById('sec_hermes_dashboard_basic_auth_secret');

    const backupFields = {
        passphrase: document.getElementById('sec_borg_pass'),
        endpoint: document.getElementById('sec_s3_endpoint'),
        region: document.getElementById('sec_s3_region'),
        bucket: document.getElementById('sec_s3_bucket'),
        accessKey: document.getElementById('sec_s3_access_key'),
        secretKey: document.getElementById('sec_s3_secret_key'),
        workloadPrefix: document.getElementById('sec_s3_workload_prefix'),
        edgePrefix: document.getElementById('sec_s3_edge_prefix'),
    };

    let parsedEndpoint;
    try {
        parsedEndpoint = new URL(s3Endpoint);
    } catch {
        alert('Укажите корректный HTTPS endpoint Timeweb S3.');
        backupFields.endpoint.focus();
        return;
    }
    if (parsedEndpoint.protocol !== 'https:' || !parsedEndpoint.hostname ||
        (parsedEndpoint.pathname !== '/' && parsedEndpoint.pathname !== '') ||
        parsedEndpoint.search || parsedEndpoint.hash) {
        alert('Endpoint S3 должен быть HTTPS-адресом без пути, query-параметров и fragment.');
        backupFields.endpoint.focus();
        return;
    }
    if (borgPass.length < 20 || !borgPass.trim()) {
        alert('Фраза шифрования Borg должна содержать не менее 20 символов.');
        backupFields.passphrase.focus();
        return;
    }
    if (![s3Region, s3Bucket, s3AccessKey, s3SecretKey].every(value => value.trim().length > 0)) {
        alert('Заполните регион, имя бакета, S3 Access Key и S3 Secret Access Key.');
        const firstEmpty = [
            [s3Region, backupFields.region],
            [s3Bucket, backupFields.bucket],
            [s3AccessKey, backupFields.accessKey],
            [s3SecretKey, backupFields.secretKey],
        ].find(([value]) => !value.trim());
        if (firstEmpty) firstEmpty[1].focus();
        return;
    }
    const safePrefix = /^[A-Za-z0-9][A-Za-z0-9_-]*$/;
    if (!safePrefix.test(s3WorkloadPrefix.trim()) || !safePrefix.test(s3EdgePrefix.trim()) ||
        s3WorkloadPrefix.trim() === s3EdgePrefix.trim()) {
        alert('Префиксы Workload и Edge должны соответствовать шаблону [A-Za-z0-9][A-Za-z0-9_-]* и различаться.');
        backupFields.workloadPrefix.focus();
        return;
    }
    const safeBucket = /^[a-z0-9][a-z0-9.-]{1,61}[a-z0-9]$/;
    if (!safeBucket.test(s3Bucket.trim()) || s3Bucket.includes('..')) {
        alert('Укажите корректное S3-имя бакета (латиница в нижнем регистре, цифры, точки и дефисы; без пути).');
        backupFields.bucket.focus();
        return;
    }

    let parsedHermesEndpoint;
    try {
        parsedHermesEndpoint = new URL(hermesApiBaseUrl);
    } catch {
        alert('Укажите корректный Base URL API для Hermes.');
        document.getElementById('sec_hermes_api_base_url').focus();
        return;
    }
    if (!['https:', 'http:'].includes(parsedHermesEndpoint.protocol) ||
        !parsedHermesEndpoint.hostname || parsedHermesEndpoint.username ||
        parsedHermesEndpoint.password || parsedHermesEndpoint.search ||
        parsedHermesEndpoint.hash || hermesApiBaseUrl.includes('\n') || hermesApiBaseUrl.includes('\r')) {
        alert('Hermes API Base URL должен быть HTTP(S)-адресом без логина, пароля, query-параметров и fragment.');
        document.getElementById('sec_hermes_api_base_url').focus();
        return;
    }
    if (!hermesApiKey || hermesApiKey.includes('\n') || hermesApiKey.includes('\r')) {
        alert('Введите API-ключ для Hermes. Ключ не должен содержать переносы строк.');
        document.getElementById('sec_hermes_api_key').focus();
        return;
    }
    if (!hermesModel || hermesModel.includes('\n') || hermesModel.includes('\r')) {
        alert('Введите идентификатор модели Hermes без переносов строк.');
        document.getElementById('sec_hermes_model').focus();
        return;
    }

    let hermesDashboardSecret = hermesSecretField.value.trim();
    if (!/^[a-f0-9]{64}$/i.test(hermesDashboardSecret)) {
        if (!window.crypto || typeof window.crypto.getRandomValues !== 'function') {
            alert('Не удалось автоматически создать секрет сессии Hermes. Используйте защищённую версию установщика и повторите попытку.');
            return;
        }
        const secretBytes = new Uint8Array(32);
        window.crypto.getRandomValues(secretBytes);
        hermesDashboardSecret = Array.from(secretBytes, byte => byte.toString(16).padStart(2, '0')).join('');
        hermesSecretField.value = hermesDashboardSecret;
    }

    const formattedPrivateKey = privatesshKey
        .split('\n')
        .map(line => '    ' + line)
        .join('\n');

    generatedYamlString = `vps_public_ip: ${yamlQuote(vpsIp)}
vps_user: ${yamlQuote(vpsUser)}
vps_root_password: ${yamlQuote(vpsRootPass)}
local_private_ip: ${yamlQuote(localIp)}
local_user: ${yamlQuote(localUser)}
local_root_password: ${yamlQuote(localRootPass)}

server_name: ${yamlQuote(serverName)}
admin_user: ${yamlQuote(adminUser)}
admin_password: ${yamlQuote(adminPass)}
email: ${yamlQuote(email)}

telegram_token: ${yamlQuote(tgToken)}
telegram_chat_id: ${yamlQuote(tgChatId)}

webnames_apikey: ${yamlQuote(webnamesKey)}

postgres_user: ${yamlQuote(pgUser)}
postgres_password: ${yamlQuote(pgPass)}
nextcloud_redis_pass: ${yamlQuote(redisPass)}

secret_vaultwarden_password: ${yamlQuote(vwAdminPass)}
hbox_pepper: ${yamlQuote(hboxPepper)}
ssh_public_key: ${yamlQuote(publicsshKey)}
ssh_private_key: |
${formattedPrivateKey}
borgmatic_encryption_passphrase: ${yamlQuote(borgPass)}
backup_disk_uuid: ${yamlQuote(diskUuid)}
offsite_s3_endpoint: ${yamlQuote(s3Endpoint)}
offsite_s3_region: ${yamlQuote(s3Region)}
offsite_s3_bucket: ${yamlQuote(s3Bucket)}
offsite_s3_access_key_id: ${yamlQuote(s3AccessKey)}
offsite_s3_secret_access_key: ${yamlQuote(s3SecretKey)}
offsite_s3_workload_prefix: ${yamlQuote(s3WorkloadPrefix)}
offsite_s3_edge_prefix: ${yamlQuote(s3EdgePrefix)}

hermes_api_base_url: ${yamlQuote(hermesApiBaseUrl)}
hermes_api_key: ${yamlQuote(hermesApiKey)}
hermes_model: ${yamlQuote(hermesModel)}
hermes_dashboard_basic_auth_secret: ${yamlQuote(hermesDashboardSecret)}
`;

    document.getElementById('yaml-preview').innerText = generatedYamlString;

    const btnStep6Back = document.getElementById('btn-step6-back');
    if (btnStep6Back) {
        btnStep6Back.setAttribute('onclick', 'goToStep(5)');
    }
    
    goToStep(5);
}

window.copyYamlToClipboard = async function() {
    const btn = document.getElementById('btn-copy-yaml');
    const btnText = document.getElementById('copy-btn-text');
    if (!generatedYamlString) return;

    try {
        await navigator.clipboard.writeText(generatedYamlString);
        btn.classList.add('copied');
        btnText.innerText = 'Скопировано ✓';
    } catch (err) {
        btnText.innerText = 'Ошибка копирования';
        console.error('Не удалось скопировать secrets.yml в буфер обмена:', err);
    } finally {
        setTimeout(() => {
            btn.classList.remove('copied');
            btnText.innerText = 'Копировать';
        }, 2000);
    }
}

window.saveYamlAndFinish = async function() {
    try {
        const result = await SaveSecrets(generatedYamlString);
        if (result.success) {
            goToStep(6);
        } else {
            alert(`Ошибка записи файла: ${result.message}`);
        }
    } catch (err) {
        alert(`Критическая ошибка бэкенда при сохранении: ${err}`);
    }
}

window.closeInstaller = function() {
    CloseApp();
}

let selectedOptionNum = 0;

window.selectDeployOption = function(num) {
    selectedOptionNum = num;
    
    document.querySelectorAll('.deploy-option-card').forEach(card => {
        card.classList.remove('selected');
    });
    
    const selectedCard = document.getElementById(`opt-${num}`);
    if (selectedCard) {
        selectedCard.classList.add('selected');
    }

    const btnNext = document.getElementById('btn-deploy-next');
    if (btnNext) {
        btnNext.disabled = false;
    }
}

window.sendEnterToAnsible = async function() {
    try {
        const result = await SendEnter();
        if (!result.success) {
            console.warn(result.message);
        }
    } catch (err) {
        console.error("Ошибка при отправке Enter:", err);
    }
}

window.confirmDeployment = function() {
    if (selectedOptionNum === 0) return;
    
    const optionTexts = {
        1: "Пункт 1. Полный деплой (Локальный + VPS)",
        2: "Пункт 2. Запуск сервисов (Локальный + VPS)",
        3: "Пункт 3. Полный деплой (Локальный)",
        4: "Пункт 4. Запуск сервисов (Локальный)",
        5: "Пункт 5. Полный деплой (VPS)",
        6: "Пункт 6. Запуск сервисов (VPS)"
    };
    
    const confirmTextEl = document.getElementById('confirm-option-text');
    if (confirmTextEl) {
        confirmTextEl.innerText = optionTexts[selectedOptionNum] || `Сценарий №${selectedOptionNum}`;
    }
    
    const isVerbose = document.getElementById('verbose-logs-toggle').checked;
    const confirmVerboseEl = document.getElementById('confirm-verbose-text');
    if (confirmVerboseEl) {
        confirmVerboseEl.innerText = isVerbose ? "Да (-vvv)" : "Нет";
    }
    
    goToStep(7);
}

window.startDeployment = async function() {
    const logContainer = document.getElementById('deploy-logs-container');
    const logPre = document.getElementById('deploy-logs');
    const btnRun = document.getElementById('btn-deploy-run');
    const btnBack = document.getElementById('btn-deploy-back');
    const btnExit = document.getElementById('btn-deploy-exit');
    const btnEnter = document.getElementById('btn-deploy-enter');
    const isVerbose = document.getElementById('verbose-logs-toggle').checked;

    logContainer.style.display = 'block';
    logPre.innerText = 'Подготовка окружения и инициализация процесса деплоя...\n';
    
    btnRun.disabled = true;
    btnBack.disabled = true;
    if (btnExit) btnExit.style.display = 'none';
    if (btnEnter) btnEnter.style.display = 'block';

    const postDeployGuide = document.getElementById('post-deploy-guide');
    if (postDeployGuide) postDeployGuide.style.display = 'none';

    function addLog(message) {
        const logPre = document.getElementById('deploy-logs');
        const isAtBottom = (logPre.scrollHeight - logPre.clientHeight) <= (logPre.scrollTop + 5);
        logPre.innerText += message;
        if (isAtBottom) {
            logPre.scrollTop = logPre.scrollHeight;
        }
    }

    if (!window.deployLogListened) {
        EventsOn('deploy-log', (message) => {
            addLog(message);
        });
        window.deployLogListened = true;
    }

    try {
        const result = await RunDeployment(selectedOptionNum, isVerbose);
        
        if (btnEnter) btnEnter.style.display = 'none';

        if (result.success) {
            logPre.innerText += `\n[ УСПЕХ ]: ${result.message}\n`;
            logPre.style.borderColor = '#818cf8';
            btnRun.style.display = 'none';
            btnBack.style.display = 'none';
            if (btnExit) {
                btnExit.style.display = 'block';
                btnExit.className = 'btn-primary';
            }

            const guide = document.getElementById('post-deploy-guide');
            if (guide) guide.style.display = 'block';
        } else {
            logPre.innerText += `\n[ ОШИБКА ]: Деплой завершился неудачей.\nПричина: ${result.message}\n`;
            logPre.style.borderColor = '#e5484d';
            btnRun.disabled = false;
            btnBack.disabled = false;
            if (btnExit) btnExit.style.display = 'block';
        }
    } catch (err) {
        if (btnEnter) btnEnter.style.display = 'none';
        logPre.innerText += `\n[ КРИТИЧЕСКАЯ ОШИБКА СИСТЕМЫ ]: Ошибка вызова бэкенда:\n${err}\n`;
        logPre.style.borderColor = '#e5484d';

        btnRun.disabled = false;
        btnBack.disabled = false;
        if (btnExit) btnExit.style.display = 'block';
    }
}

window.closeWindow = function() {
    CloseApp();
}

window.minimizeWindow = function() {
    MinimizeApp();
}

window.maximizeWindow = function() {
    MaximizeApp();
}

updateStepProgress(1);