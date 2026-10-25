import './style.css';
import { CheckDocker, InstallProject, GetProjectVersions, ResolveProjectVersion, SaveSecrets, PreviewSecrets, CloseApp, MinimizeApp, MaximizeApp, RunDeployment, CheckSecrets, LoadSecrets, SendEnter } from '../wailsjs/go/main/App.js';
import { EventsOn } from '../wailsjs/runtime/runtime.js';

let generatedYamlString = "";
let sourceSecretValues = {};
let sourceSecretIsTemplate = true;

const STEP_TITLES = {
    1: 'Выбор версии проекта',
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
    installText.innerText = 'Проверяем выбранную версию и загружаем её во временный каталог...';
    installText.style.color = 'var(--text-secondary)';
    btnFinish.disabled = true;
    btnFinish.style.display = 'inline-block';
    btnRetry.style.display = 'none';
    btnEditSecrets.style.display = 'none';

    try {
        const selection = getSelectedProjectVersion();
        if (!selection.valid) {
            spinner.style.display = 'none';
            installText.innerText = selection.message || 'Сначала выберите и проверьте версию проекта.';
            installText.style.color = 'var(--danger)';
            btnFinish.style.display = 'none';
            btnRetry.style.display = 'inline-block';
            return;
        }
        const migrationConsent = document.getElementById('replace-legacy-project');
        const replaceLegacy = Boolean(migrationConsent && migrationConsent.checked);
        const result = await InstallProject(selection.mode, selection.ref, selection.expectedSHA, replaceLegacy);
        
        if (result.success) {
            const secretsCheck = await CheckSecrets();
            const existingSecrets = await LoadSecrets();
            if (!existingSecrets.success) {
                throw new Error(existingSecrets.message || 'Не удалось прочитать и проверить secrets.yml или его шаблон.');
            }
            populateSecretsForm(existingSecrets.values, existingSecrets.isTemplate);

            spinner.style.display = 'none';

            if (secretsCheck.success) {
                const readHermesSecret = (key) => String(existingSecrets.values?.[key] ?? '');
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

                installText.innerText = '✓ ' + result.message + '\n\nКонфигурация найдена: файл secrets.yml уже существует. Шаги создания и настройки будут пропущены.';
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
    vps_ssh_password: 'sec_vps_ssh_pass',
    vps_sudo_password: 'sec_vps_sudo_pass',
    local_private_ip: 'sec_local_ip',
    local_user: 'sec_local_user',
    local_root_password: 'sec_local_root_pass',
    local_ssh_password: 'sec_local_ssh_pass',
    local_sudo_password: 'sec_local_sudo_pass',
    server_name: 'sec_server_name',
    admin_user: 'sec_admin_user',
    hbox_pepper: 'sec_hbox_pepper',
    adguard_password_salt: 'sec_adguard_password_salt',
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
    borgmatic_version: 'sec_borgmatic_version',
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

function isTemplatePlaceholder(value) {
    const text = String(value ?? '').trim().toLowerCase();
    return text.startsWith('replace_with_') ||
        text.includes('replace_with_your_') ||
        text === 'ssh-ed25519 replace_with_your_public_key';
}

function populateSecretsForm(values, isTemplate = false) {
    const source = values && typeof values === 'object' ? values : {};
    // Keep the parsed source values as the baseline. Template placeholders are
    // hidden in the UI so the user is not tricked into treating them as secrets;
    // they will count as changes when the form is previewed.
    sourceSecretValues = { ...source };
    sourceSecretIsTemplate = Boolean(isTemplate);

    for (const [key, fieldId] of Object.entries(SECRETS_FIELD_MAP)) {
        const element = document.getElementById(fieldId);
        if (!element) continue;
        if (!Object.prototype.hasOwnProperty.call(source, key)) {
            element.value = '';
            continue;
        }
        const originalValue = String(source[key] ?? '');
        if (sourceSecretIsTemplate && isTemplatePlaceholder(originalValue)) {
            const fallback = String(element.defaultValue ?? '');
            element.value = isTemplatePlaceholder(fallback) ? '' : fallback;
        } else {
            element.value = originalValue;
        }
    }
}

function collectSecretFormValues(normalized = {}) {
    const values = {};
    for (const [key, fieldId] of Object.entries(SECRETS_FIELD_MAP)) {
        const element = document.getElementById(fieldId);
        if (!element) continue;
        values[key] = Object.prototype.hasOwnProperty.call(normalized, key)
            ? normalized[key]
            : String(element.value ?? '');
    }
    return values;
}

// Only changed, known fields are sent to the backend. Unknown fields and
// untouched YAML nodes never make a round trip through the HTML form.
function buildChangedSecretUpdates(values) {
    const updates = {};
    for (const [key, value] of Object.entries(values)) {
        const current = String(value ?? '');
        const existed = Object.prototype.hasOwnProperty.call(sourceSecretValues, key);
        const previous = existed ? String(sourceSecretValues[key] ?? '') : '';
        if (existed && current === previous) continue;
        if (!existed && current === '') continue;
        updates[key] = current;
    }
    return updates;
}

window.editExistingSecrets = async function() {
    try {
        const result = await LoadSecrets();
        if (result.success) {
            populateSecretsForm(result.values, result.isTemplate);
            goToStep(4);
        } else {
            alert(`Не удалось загрузить текущую конфигурацию: ${result.message}`);
        }
    } catch (err) {
        alert(`Критическая ошибка бэкенда при загрузке конфигурации: ${err}`);
    }
}

window.generateAndReviewYaml = async function() {
    const getValue = (id) => String(document.getElementById(id)?.value ?? '');
    const vpsIp = getValue('sec_vps_ip');
    const vpsUser = getValue('sec_vps_user');
    const vpsRootPass = getValue('sec_vps_root_pass');
    const localIp = getValue('sec_local_ip');
    const localUser = getValue('sec_local_user');
    const localRootPass = getValue('sec_local_root_pass');
    const hboxPepper = getValue('sec_hbox_pepper');

    const serverName = getValue('sec_server_name');
    const adminUser = getValue('sec_admin_user');
    const adminPass = getValue('sec_admin_pass');
    const email = getValue('sec_email');

    const tgToken = getValue('sec_tg_token');
    const tgChatId = getValue('sec_tg_chat_id');
    const webnamesKey = getValue('sec_webnames');

    const pgUser = getValue('sec_pg_user');
    const pgPass = getValue('sec_pg_pass');
    const redisPass = getValue('sec_redis_pass');

    const vwAdminPass = getValue('sec_vw_admin_pass');
    const publicsshKey = getValue('sec_public_ssh_key');
    const privatesshKey = getValue('sec_private_ssh_key');
    const borgPass = getValue('sec_borg_pass');
    const diskUuid = getValue('sec_disk_uuid');
    const s3Endpoint = getValue('sec_s3_endpoint').trim().replace(/\/+$/, '');
    const s3Region = getValue('sec_s3_region').trim();
    const s3Bucket = getValue('sec_s3_bucket').trim();
    const s3AccessKey = getValue('sec_s3_access_key').trim();
    const s3SecretKey = getValue('sec_s3_secret_key').trim();
    const s3WorkloadPrefix = getValue('sec_s3_workload_prefix').trim();
    const s3EdgePrefix = getValue('sec_s3_edge_prefix').trim();

    const hermesApiBaseUrl = getValue('sec_hermes_api_base_url').trim().replace(/\/+$/, '');
    const hermesApiKey = getValue('sec_hermes_api_key').trim();
    const hermesModel = getValue('sec_hermes_model').trim();
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

    const configIsTemplate = sourceSecretIsTemplate;

    const backupKeyNames = [
        'offsite_s3_endpoint', 'offsite_s3_region', 'offsite_s3_bucket',
        'offsite_s3_access_key_id', 'offsite_s3_secret_access_key',
        'offsite_s3_workload_prefix', 'offsite_s3_edge_prefix',
        'borgmatic_encryption_passphrase',
    ];
    const backupGroupChanged = backupKeyNames.some(key => {
        const current = String(collectSecretFormValues({
            offsite_s3_endpoint: s3Endpoint,
            offsite_s3_region: s3Region,
            offsite_s3_bucket: s3Bucket,
            offsite_s3_access_key_id: s3AccessKey,
            offsite_s3_secret_access_key: s3SecretKey,
            offsite_s3_workload_prefix: s3WorkloadPrefix,
            offsite_s3_edge_prefix: s3EdgePrefix,
        })[key] ?? '');
        return current !== String(sourceSecretValues[key] ?? '');
    });

    if (configIsTemplate || backupGroupChanged) {
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
        const requiredS3 = [s3Region, s3Bucket, s3AccessKey, s3SecretKey];
        if (!requiredS3.every(value => value.trim().length > 0)) {
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
        if (!safePrefix.test(s3WorkloadPrefix) || !safePrefix.test(s3EdgePrefix) ||
            s3WorkloadPrefix === s3EdgePrefix) {
            alert('Префиксы Workload и Edge должны соответствовать шаблону [A-Za-z0-9][A-Za-z0-9_-]* и различаться.');
            backupFields.workloadPrefix.focus();
            return;
        }
        const safeBucket = /^[a-z0-9][a-z0-9.-]{1,61}[a-z0-9]$/;
        if (!safeBucket.test(s3Bucket) || s3Bucket.includes('..')) {
            alert('Укажите корректное S3-имя бакета (латиница в нижнем регистре, цифры, точки и дефисы; без пути).');
            backupFields.bucket.focus();
            return;
        }
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

    let hermesDashboardSecret = String(hermesSecretField?.value ?? '').trim();
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

    const normalizedValues = collectSecretFormValues({
        offsite_s3_endpoint: s3Endpoint,
        offsite_s3_region: s3Region,
        offsite_s3_bucket: s3Bucket,
        offsite_s3_access_key_id: s3AccessKey,
        offsite_s3_secret_access_key: s3SecretKey,
        offsite_s3_workload_prefix: s3WorkloadPrefix,
        offsite_s3_edge_prefix: s3EdgePrefix,
        hermes_api_base_url: hermesApiBaseUrl,
        hermes_api_key: hermesApiKey,
        hermes_model: hermesModel,
        hermes_dashboard_basic_auth_secret: hermesDashboardSecret,
    });
    const updates = buildChangedSecretUpdates(normalizedValues);

    const previewButton = document.querySelector('#step-4 .button-group .btn-primary');
    const originalButtonText = previewButton ? previewButton.innerText : '';
    if (previewButton) {
        previewButton.disabled = true;
        previewButton.innerText = 'Проверка YAML...';
    }

    try {
        const preview = await PreviewSecrets(updates);
        if (!preview.success) {
            alert('Итоговый secrets.yml не прошёл проверку: ' + preview.message);
            return;
        }
        generatedYamlString = preview.content;
        document.getElementById('yaml-preview').innerText = generatedYamlString;

        const btnStep6Back = document.getElementById('btn-step6-back');
        if (btnStep6Back) {
            btnStep6Back.setAttribute('onclick', 'goToStep(5)');
        }
        goToStep(5);
    } catch (err) {
        alert('Не удалось объединить и проверить конфигурацию YAML: ' + err);
    } finally {
        if (previewButton) {
            previewButton.disabled = false;
            previewButton.innerText = originalButtonText || 'Продолжить';
        }
    }
};

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



let projectVersionCatalog = null;
let projectInstallState = null;
let projectVersionsLoaded = false;
let legacyInstallBlocked = false;
let resolvedProjectCommitSHA = "";
let projectCommitInputAtResolution = "";
let projectCommitPreviewRequest = 0;

function isFullSHA(value) {
    return /^[a-f0-9]{40}$/i.test(String(value || '').trim());
}

function selectedVersionOption() {
    const select = document.getElementById('project-version-select');
    if (!select || !select.selectedOptions || !select.selectedOptions[0]) return null;
    return select.selectedOptions[0];
}

function setProjectVersionContinueEnabled(enabled) {
    const button = document.getElementById('btn-version-continue');
    const consent = document.getElementById('replace-legacy-project');
    const migrationAllowed = !legacyInstallBlocked || Boolean(consent && consent.checked);
    if (button) button.disabled = !enabled || !projectVersionsLoaded || !migrationAllowed;
}

function getSelectedProjectVersion() {
    if (!projectVersionsLoaded) {
        return { valid: false, message: 'Список версий ещё не загружен. Обновите список и повторите.' };
    }
    const migrationConsent = document.getElementById('replace-legacy-project');
    if (legacyInstallBlocked && !(migrationConsent && migrationConsent.checked)) {
        return { valid: false, message: 'Существующая установка не содержит метаданных. Подтвердите резервное сохранение старого каталога, прежде чем продолжить.' };
    }
    const useCommit = document.getElementById('use-project-commit-sha');
    if (useCommit && useCommit.checked) {
        const input = document.getElementById('project-commit-sha');
        const rawSHA = String(input ? input.value : '').trim();
        if (!isFullSHA(resolvedProjectCommitSHA) || projectCommitInputAtResolution !== rawSHA) {
            return { valid: false, message: 'Сначала проверьте указанный commit SHA — это делается после потери фокуса поля.' };
        }
        return { valid: true, mode: 'commit', ref: rawSHA, expectedSHA: resolvedProjectCommitSHA };
    }

    const option = selectedVersionOption();
    if (!option || !option.dataset.ref || !isFullSHA(option.dataset.sha)) {
        return { valid: false, message: 'У выбранного источника пока нет подтверждённого commit SHA. Обновите список версий.' };
    }
    if (option.value === 'release') {
        return { valid: true, mode: 'release', ref: option.dataset.ref, expectedSHA: option.dataset.sha };
    }
    if (option.value === 'dev') {
        return { valid: true, mode: 'dev', ref: 'dev', expectedSHA: option.dataset.sha };
    }
    if (option.value.startsWith('tag:')) {
        return { valid: true, mode: 'tag', ref: option.dataset.ref, expectedSHA: option.dataset.sha };
    }
    return { valid: false, message: 'Неизвестный вариант версии.' };
}

function addProjectVersionOption(select, value, label, ref, sha, mode) {
    const option = document.createElement('option');
    option.value = value;
    option.textContent = label;
    option.dataset.ref = ref || '';
    option.dataset.sha = sha || '';
    option.dataset.mode = mode || '';
    select.appendChild(option);
}

function populateProjectVersionChoices(result, preserveChoice) {
    const select = document.getElementById('project-version-select');
    const previousValue = preserveChoice ? select.value : '';
    select.innerHTML = '';

    const latest = result.latestRelease;
    addProjectVersionOption(
        select,
        'release',
        'Последний стабильный релиз — ' + latest.ref,
        latest.ref,
        latest.commitSHA,
        'release'
    );

    const dev = result.dev;
    addProjectVersionOption(
        select,
        'dev',
        'Ветка dev — последние изменения (нестабильно)',
        dev.ref,
        dev.commitSHA,
        'dev'
    );

    if (Array.isArray(result.tags) && result.tags.length) {
        const group = document.createElement('optgroup');
        group.label = 'Опубликованные теги';
        for (const tag of result.tags) {
            const option = document.createElement('option');
            option.value = 'tag:' + tag.ref;
            option.textContent = tag.ref + ' — ' + tag.commitSHA.slice(0, 8);
            option.dataset.ref = tag.ref;
            option.dataset.sha = tag.commitSHA;
            option.dataset.mode = 'tag';
            group.appendChild(option);
        }
        select.appendChild(group);
    }

    const availableValues = Array.from(select.options).map(option => option.value);
    if (preserveChoice && availableValues.includes(previousValue)) {
        select.value = previousValue;
    } else {
        select.value = 'release';
    }
    select.disabled = false;
}

function renderInstalledProjectState(state, initialLoad) {
    const note = document.getElementById('project-installed-note');
    const useCommit = document.getElementById('use-project-commit-sha');
    const shaInput = document.getElementById('project-commit-sha');
    const select = document.getElementById('project-version-select');
    if (!note) return;

    legacyInstallBlocked = Boolean(state && state.exists && !state.knownVersion);
    const consentWrapper = document.getElementById('legacy-migration-consent-wrapper');
    const consentCheckbox = document.getElementById('replace-legacy-project');
    if (legacyInstallBlocked) {
        note.style.display = 'block';
        note.classList.add('is-warning');
        note.textContent = 'Найдена прежняя папка ServeHub-2-dev без корректного .servehub-version.json. Её фактический commit определить нельзя. Для миграции нужно отдельное подтверждение: установщик сохранит всю прежнюю папку в резервной копии и перенесёт secrets.yml, если он является обычным файлом.';
        if (consentWrapper) consentWrapper.style.display = 'flex';
        if (consentCheckbox && initialLoad) consentCheckbox.checked = false;
    } else if (state && state.exists && state.knownVersion) {
        if (consentWrapper) consentWrapper.style.display = 'none';
        if (consentCheckbox) consentCheckbox.checked = false;
        const meta = state.metadata;
        const sha = meta.commit_sha;
        note.style.display = 'block';
        note.classList.remove('is-warning');
        note.textContent = 'В каталоге уже установлено: ' + meta.display_name + ', commit ' + sha +
            '. Повторный запуск закреплён на этом SHA. Чтобы выбрать другой релиз или dev, сначала снимите флажок «Указать конкретный commit SHA», затем выберите источник.';
        if (initialLoad) {
            useCommit.checked = true;
            shaInput.value = sha;
            resolvedProjectCommitSHA = sha;
            projectCommitInputAtResolution = sha;
        }
    } else {
        note.style.display = 'none';
        note.textContent = '';
        if (consentWrapper) consentWrapper.style.display = 'none';
        if (consentCheckbox) consentCheckbox.checked = false;
    }
    if (useCommit) useCommit.disabled = false;
    if (select) select.disabled = Boolean(useCommit && useCommit.checked);
    if (shaInput) shaInput.disabled = !(useCommit && useCommit.checked);
}

window.initializeProjectVersions = async function(preserveChoice = false) {
    const select = document.getElementById('project-version-select');
    const summary = document.getElementById('project-version-summary');
    const continueButton = document.getElementById('btn-version-continue');
    const refreshButton = document.getElementById('btn-refresh-project-versions');
    if (refreshButton) refreshButton.disabled = true;
    if (continueButton) continueButton.disabled = true;
    if (!preserveChoice && summary) summary.textContent = 'Получаем опубликованные релизы, теги и commit SHA с GitHub...';

    try {
        const result = await GetProjectVersions();
        if (!result.success) throw new Error(result.message || 'GitHub не вернул список версий.');
        const previousSHA = resolvedProjectCommitSHA;
        projectVersionCatalog = result;
        projectInstallState = result.installed;
        populateProjectVersionChoices(result, preserveChoice);
        renderInstalledProjectState(result.installed, !preserveChoice);
        projectVersionsLoaded = true;
        if (preserveChoice && previousSHA) {
            resolvedProjectCommitSHA = previousSHA;
        }
        window.updateProjectVersionPreview();
    } catch (error) {
        projectVersionsLoaded = false;
        if (summary) summary.textContent = 'Не удалось получить список версий: ' + String(error);
        if (continueButton) continueButton.disabled = true;
    } finally {
        if (refreshButton) refreshButton.disabled = false;
    }
};

window.refreshProjectVersions = async function() {
    await window.initializeProjectVersions(true);
};

window.toggleProjectCommitMode = function() {
    const enabled = document.getElementById('use-project-commit-sha').checked;
    const select = document.getElementById('project-version-select');
    const input = document.getElementById('project-commit-sha');
    select.disabled = enabled || !projectVersionsLoaded;
    input.disabled = !enabled;
    if (enabled) {
        input.focus();
        resolvedProjectCommitSHA = '';
        projectCommitInputAtResolution = '';
    }
    window.updateProjectVersionPreview();
    if (enabled && /^[a-f0-9]{7,40}$/i.test(input.value.trim())) {
        window.previewProjectCommit();
    }
};

window.markProjectCommitChanged = function() {
    resolvedProjectCommitSHA = '';
    projectCommitInputAtResolution = '';
    const summary = document.getElementById('project-commit-summary');
    if (summary) summary.textContent = 'SHA изменён. Потеряйте фокус поля, чтобы проверить commit через GitHub.';
    window.updateProjectVersionPreview();
};

window.previewProjectCommit = async function() {
    const input = document.getElementById('project-commit-sha');
    const summary = document.getElementById('project-commit-summary');
    if (!input || !summary) return;
    const rawSHA = String(input.value || '').trim();
    if (!/^[a-f0-9]{7,40}$/i.test(rawSHA)) {
        summary.textContent = 'Введите от 7 до 40 шестнадцатеричных символов commit SHA.';
        resolvedProjectCommitSHA = '';
        projectCommitInputAtResolution = '';
        window.updateProjectVersionPreview();
        return;
    }
    const requestId = ++projectCommitPreviewRequest;
    summary.textContent = 'Проверяем commit SHA через GitHub...';
    resolvedProjectCommitSHA = '';
    projectCommitInputAtResolution = '';
    setProjectVersionContinueEnabled(false);
    try {
        const result = await ResolveProjectVersion('commit', rawSHA, '');
        if (requestId !== projectCommitPreviewRequest || rawSHA !== input.value.trim()) return;
        if (!result.success) throw new Error(result.message || 'Commit не найден.');
        resolvedProjectCommitSHA = result.commitSHA;
        projectCommitInputAtResolution = rawSHA;
        summary.textContent = 'Проверенный commit SHA: ' + result.commitSHA;
        window.updateProjectVersionPreview();
    } catch (error) {
        if (requestId !== projectCommitPreviewRequest) return;
        summary.textContent = 'Не удалось проверить commit: ' + String(error);
        window.updateProjectVersionPreview();
    }
};

window.updateProjectVersionPreview = function() {
    const summary = document.getElementById('project-version-summary');
    const warning = document.getElementById('project-version-warning');
    const useCommit = document.getElementById('use-project-commit-sha');
    if (!summary || !warning) return;
    const migrationConsent = document.getElementById('replace-legacy-project');
    if (legacyInstallBlocked && !(migrationConsent && migrationConsent.checked)) {
        summary.textContent = 'Существующая папка не содержит закреплённой версии. Подтвердите её резервное сохранение, чтобы продолжить с выбранным commit.';
        setProjectVersionContinueEnabled(false);
        return;
    }
    if (useCommit && useCommit.checked) {
        const input = document.getElementById('project-commit-sha');
        const rawSHA = input ? input.value.trim() : '';
        if (isFullSHA(resolvedProjectCommitSHA) && projectCommitInputAtResolution === rawSHA) {
            summary.textContent = 'Будет установлен commit ' + resolvedProjectCommitSHA + '.';
            warning.textContent = legacyInstallBlocked
                ? 'Будет установлен указанный commit. Существующая установка без метаданных будет сохранена целиком в резервной папке; secrets.yml будет перенесён, если это обычный файл.'
                : 'При указании SHA установщик использует именно этот commit, даже если ветки и теги позже изменятся.';
            setProjectVersionContinueEnabled(true);
        } else {
            summary.textContent = 'Для продолжения требуется проверить введённый commit SHA.';
            setProjectVersionContinueEnabled(false);
        }
        return;
    }
    const option = selectedVersionOption();
    if (!option || !option.dataset.sha) {
        summary.textContent = 'У выбранного источника пока нет подтверждённого commit SHA.';
        setProjectVersionContinueEnabled(false);
        return;
    }
    summary.textContent = 'Будет установлен источник ' + (option.dataset.ref || option.textContent) +
        ' на commit ' + option.dataset.sha + '.';
    if (legacyInstallBlocked) {
        warning.textContent = 'Выбранный commit заменит папку без известной версии. Прежняя папка останется в резервной копии; секреты будут перенесены безопасно. Проверьте, что выбранная версия совместима с вашей инфраструктурой.';
    } else if (option.value === 'dev') {
        warning.textContent = 'Ветка dev может быть нестабильной и содержит незавершённые изменения. SHA фиксируется до загрузки; если ветка изменится, установка остановится, а не переключится незаметно.';
    } else {
        warning.textContent = 'Выбранный релиз или тег может отличаться от ветки dev по сервисам и архитектуре. Установщик развернёт выбранную версию и закрепит тот же commit на серверах.';
    }
    setProjectVersionContinueEnabled(isFullSHA(option.dataset.sha));
};

window.continueFromVersionSelection = function() {
    const selection = getSelectedProjectVersion();
    if (!selection.valid) {
        const summary = document.getElementById('project-version-summary');
        if (summary) summary.textContent = selection.message;
        return;
    }
    window.goToStep(2);
};


updateStepProgress(1);
window.initializeProjectVersions();