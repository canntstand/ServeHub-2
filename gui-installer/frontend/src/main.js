import './style.css';
import { CheckDocker, GetProjectVersions, ResolveProjectVersion, GetProjectInstallDirectory, InspectProjectInstallDirectory, SetProjectInstallDirectory, ChooseProjectInstallDirectory, PerformProjectAction, RecoverProjectInstallation, CancelProjectInstallation, SaveSecrets, PreviewSecrets, GenerateSecretDefaults, ValidateDeploymentSecrets, CloseApp, MinimizeApp, MaximizeApp, RunDeployment, CheckSecrets, LoadSecrets, SendEnter } from '../wailsjs/go/main/App.js';
import { EventsOn } from '../wailsjs/runtime/runtime.js';

let generatedYamlString = "";
let generatedSecretUpdates = {};
let generatedPreviewValues = {};
let sourceSecretValues = {};
let sourceSecretIsTemplate = true;
let generatedSecretDefaultsPending = false;
let secretFormInitialized = false;
let secretFormProjectSHA = "";
let secretFormProjectDirectory = "";
let privateSSHKeyValue = "";
let privateSSHKeyVisible = false;
let privateSSHKeyToggleButton = null;

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
    const progress = document.getElementById('step-progress');
    if (progress) progress.style.display = stepNumber === 0 ? 'none' : '';
    document.body.classList.toggle('is-welcome-screen', stepNumber === 0);
    if (stepNumber < 1 || stepNumber > totalSteps) return;

    const fill = document.getElementById('step-progress-fill');
    const track = document.querySelector('.step-progress-track');
    const currentEl = document.getElementById('step-progress-current');
    const titleEl = document.getElementById('step-progress-title');

    if (fill) {
        const pct = ((stepNumber - 1) / (totalSteps - 1)) * 100;
        fill.style.width = `${pct}%`;
        if (track) track.setAttribute('aria-valuenow', String(Math.round(pct)));
    }
    if (currentEl) currentEl.innerText = String(stepNumber).padStart(2, '0');
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
    statusText.style.color = 'var(--text-secondary)';
    btnNext.disabled = true;
    btnRetry.style.display = 'none';

    await new Promise(resolve => setTimeout(resolve, 700));

    try {
        const result = await CheckDocker();

        spinner.style.display = 'none';

        if (result.success) {
            statusText.innerText = `✓ ${result.message}`;
            statusText.style.color = 'var(--success)';
            btnNext.disabled = false;
        } else {
            statusText.innerText = `✗ Ошибка: ${result.message}.`;
            statusText.style.color = 'var(--danger)';
            btnRetry.style.display = 'block';
        }
    } catch (err) {
        spinner.style.display = 'none';
        statusText.innerText = 'Критическая ошибка при вызове проверки бэкенда:\n' + err;
        statusText.style.color = 'var(--danger)';
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
        const action = selectedProjectAction();
        let selection = null;
        if (action === 'recover') {
            const recovery = await RecoverProjectInstallation();
            spinner.style.display = 'none';
            installText.innerText = recovery.message || 'Восстановление завершено.';
            installText.style.color = recovery.success ? 'var(--accent-bright)' : 'var(--danger)';
            btnFinish.innerText = 'Вернуться к выбору версии';
            btnFinish.disabled = false;
            btnFinish.setAttribute('onclick', 'goToStep(1)');
            btnRetry.style.display = 'none';
            return;
        }
        if (action !== 'use') {
            selection = getSelectedProjectVersion();
            if (!selection.valid) {
                spinner.style.display = 'none';
                installText.innerText = selection.message || 'Сначала выберите и проверьте версию проекта.';
                installText.style.color = 'var(--danger)';
                btnFinish.style.display = 'none';
                btnRetry.style.display = 'inline-block';
                return;
            }
        }
        const directoryInput = document.getElementById('project-install-directory');
        const directoryResult = await SetProjectInstallDirectory(String(directoryInput?.value ?? '').trim());
        if (!directoryResult.success) throw new Error(directoryResult.message || 'Не удалось сохранить каталог установки.');
        projectInstallDirectory = directoryResult.directory;
        projectInstallState = directoryResult.installed;
        projectInstallDirectoryVerified = true;
        const confirmReplace = Boolean(document.getElementById('confirm-replace-install')?.checked);
        const replaceLegacy = Boolean(document.getElementById('replace-legacy-project')?.checked);
        const cancelButton = document.getElementById('btn-install-cancel');
        if (cancelButton) cancelButton.style.display = action === 'install' || action === 'update' ? 'inline-block' : 'none';
        const result = await PerformProjectAction(action === 'install' && projectInstallState?.exists ? 'update' : action, selection?.mode ?? '', selection?.ref ?? '', selection?.expectedSHA ?? '', true, true);
        if (cancelButton) cancelButton.style.display = 'none';
        
        if (result.success) {
            const refreshedInstall = await GetProjectInstallDirectory();
            if (refreshedInstall.success) {
                projectInstallDirectory = refreshedInstall.directory;
                projectInstallState = refreshedInstall.installed;
                projectInstallDirectoryVerified = true;
                const pathInput = document.getElementById('project-install-directory');
                if (pathInput) pathInput.value = projectInstallDirectory;
                renderInstalledProjectState(projectInstallState, true);
                window.updateProjectActionUI();
            }
            const secretsCheck = await CheckSecrets();
            const existingSecrets = await LoadSecrets();
            if (!existingSecrets.success) {
                throw new Error(existingSecrets.message || 'Не удалось прочитать и проверить secrets.yml или его шаблон.');
            }
            // Navigation back to this step must not overwrite unsaved form edits.
            // Reload only when the selected project commit has actually changed.
            const selectedCommitSHA = selection?.expectedSHA ?? String(projectInstallState?.metadata?.commit_sha ?? '');
            if (!secretFormInitialized || secretFormProjectSHA !== selectedCommitSHA ||
                secretFormProjectDirectory !== projectInstallDirectory) {
                populateSecretsForm(existingSecrets.values, existingSecrets.isTemplate);
                await applyGeneratedSecretDefaults();
                secretFormInitialized = true;
                secretFormProjectSHA = selectedCommitSHA;
                secretFormProjectDirectory = projectInstallDirectory;
            }

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

                if (generatedSecretDefaultsPending) {
                    installText.innerText = 'В конфигурации найдены отсутствующие или шаблонные секреты. Установщик подготовил безопасные значения, но ещё не записал их в файл. Откройте форму, проверьте и сохраните конфигурацию, прежде чем переходить к деплою.';
                    installText.style.color = 'var(--accent-bright)';
                    btnFinish.disabled = false;
                    btnFinish.innerText = 'Проверить и сохранить секреты';
                    btnFinish.setAttribute('onclick', 'goToStep(4)');
                    btnEditSecrets.style.display = 'none';
                    if (btnStep6Back) {
                        btnStep6Back.setAttribute('onclick', 'goToStep(3)');
                    }
                    return;
                }

                if (!hasHermesSettings && existingSecrets.success) {
                    installText.innerText = 'Обнаружена существующая конфигурация. Заполните только отсутствующие значения, необходимые для выбранного сценария. API-ключ Hermes нужен для сценариев с VPS/Edge; сгенерированные пароли будут сохранены в secrets.yml.';
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
        const cancelButton = document.getElementById('btn-install-cancel');
        if (cancelButton) cancelButton.style.display = 'none';
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

const SENSITIVE_SECRET_FIELDS = new Set([
    'vps_root_password', 'vps_ssh_password', 'vps_sudo_password',
    'local_root_password', 'local_ssh_password', 'local_sudo_password',
    'admin_password', 'telegram_token', 'webnames_apikey',
    'postgres_password', 'nextcloud_redis_pass', 'secret_vaultwarden_password',
    'hbox_pepper', 'adguard_password_salt', 'ssh_private_key',
    'borgmatic_encryption_passphrase', 'offsite_s3_access_key_id',
    'offsite_s3_secret_access_key', 'hermes_api_key',
    'hermes_dashboard_basic_auth_secret',
]);

const PRIVATE_SSH_KEY_HIDDEN_PLACEHOLDER =
    'Приватный SSH-ключ загружен и скрыт. Нажмите «Показать ключ» для просмотра или изменения.';
const PRIVATE_SSH_KEY_EMPTY_PLACEHOLDER =
    'Ключ не задан. Нажмите «Показать ключ», чтобы вставить существующий ключ.';

function refreshPrivateSSHKeyToggle() {
    if (!privateSSHKeyToggleButton) return;
    privateSSHKeyToggleButton.textContent = privateSSHKeyVisible ? 'Скрыть ключ' : 'Показать ключ';
    privateSSHKeyToggleButton.setAttribute('aria-pressed', String(privateSSHKeyVisible));
    privateSSHKeyToggleButton.setAttribute('aria-label',
        privateSSHKeyVisible ? 'Скрыть приватный SSH-ключ' : 'Показать приватный SSH-ключ');
}

function setPrivateSSHKeyVisibility(visible) {
    const field = document.getElementById('sec_private_ssh_key');
    if (!field) {
        privateSSHKeyVisible = Boolean(visible);
        refreshPrivateSSHKeyToggle();
        return;
    }
    if (privateSSHKeyVisible) {
        privateSSHKeyValue = String(field.value ?? '');
    }
    privateSSHKeyVisible = Boolean(visible);
    field.value = privateSSHKeyVisible ? privateSSHKeyValue : '';
    field.placeholder = privateSSHKeyVisible
        ? ''
        : (privateSSHKeyValue.trim() ? PRIVATE_SSH_KEY_HIDDEN_PLACEHOLDER : PRIVATE_SSH_KEY_EMPTY_PLACEHOLDER);
    refreshPrivateSSHKeyToggle();
}

function setupSecretVisibilityToggles() {
    for (const key of SENSITIVE_SECRET_FIELDS) {
        const fieldId = SECRETS_FIELD_MAP[key];
        const field = fieldId ? document.getElementById(fieldId) : null;
        if (!field || field.type === 'hidden' || field.dataset.secretVisibilityToggle === 'ready') continue;

        const isPrivateKey = key === 'ssh_private_key';
        if (!isPrivateKey && field.tagName !== 'TEXTAREA') field.type = 'password';

        let wrapper = field.parentElement;
        if (!wrapper || !wrapper.classList.contains('password-wrapper')) {
            wrapper = document.createElement('div');
            wrapper.className = 'secret-field-control';
            if (isPrivateKey) wrapper.classList.add('secret-textarea-control');
            field.parentNode.insertBefore(wrapper, field);
            wrapper.appendChild(field);
        } else {
            wrapper.classList.add('secret-field-control');
            if (isPrivateKey) wrapper.classList.add('secret-textarea-control');
        }

        const toggle = document.createElement('button');
        toggle.type = 'button';
        toggle.className = 'secret-visibility-toggle';
        toggle.textContent = 'Показать';
        toggle.setAttribute('aria-controls', field.id);
        toggle.setAttribute('aria-pressed', 'false');
        wrapper.appendChild(toggle);

        if (isPrivateKey) {
            privateSSHKeyToggleButton = toggle;
            field.addEventListener('input', () => {
                if (privateSSHKeyVisible) privateSSHKeyValue = String(field.value ?? '');
            });
            field.value = privateSSHKeyVisible ? privateSSHKeyValue : '';
            field.placeholder = privateSSHKeyVisible
                ? ''
                : (privateSSHKeyValue.trim() ? PRIVATE_SSH_KEY_HIDDEN_PLACEHOLDER : PRIVATE_SSH_KEY_EMPTY_PLACEHOLDER);
            refreshPrivateSSHKeyToggle();
            toggle.addEventListener('click', () => {
                const show = !privateSSHKeyVisible;
                setPrivateSSHKeyVisibility(show);
                if (show) field.focus();
            });
        } else {
            toggle.setAttribute('aria-label', 'Показать секрет');
            toggle.addEventListener('click', () => {
                const show = field.type === 'password';
                field.type = show ? 'text' : 'password';
                toggle.textContent = show ? 'Скрыть' : 'Показать';
                toggle.setAttribute('aria-pressed', String(show));
                toggle.setAttribute('aria-label', show ? 'Скрыть секрет' : 'Показать секрет');
            });
        }

        field.dataset.secretVisibilityToggle = 'ready';
    }
}

function isTemplatePlaceholder(value) {
    const text = String(value ?? '').trim().toLowerCase();
    return text.startsWith('replace_with_') ||
        text.includes('replace_with_your_') ||
        text.startsWith('your_') ||
        text.includes('of_the_bot_you_created') ||
        text.includes('where_ansible_is_installed') ||
        text.startsWith('change_this_to_') ||
        text.endsWith('_secure_password') ||
        text === 'hbox_key_pepper' ||
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

        let nextValue = '';
        if (Object.prototype.hasOwnProperty.call(source, key)) {
            const originalValue = String(source[key] ?? '');
            const exampleChatID = key === 'telegram_chat_id' && originalValue === '-1001234567890';
            if (sourceSecretIsTemplate && (isTemplatePlaceholder(originalValue) || exampleChatID)) {
                const fallback = String(element.defaultValue ?? '');
                nextValue = exampleChatID || isTemplatePlaceholder(fallback) ? '' : fallback;
            } else {
                nextValue = originalValue;
            }
        }

        if (key === 'ssh_private_key') {
            // Keep the actual private key in application state; never render it
            // in the textarea unless the user explicitly reveals it.
            privateSSHKeyValue = nextValue;
            privateSSHKeyVisible = false;
            element.value = '';
            element.placeholder = privateSSHKeyValue.trim()
                ? PRIVATE_SSH_KEY_HIDDEN_PLACEHOLDER
                : PRIVATE_SSH_KEY_EMPTY_PLACEHOLDER;
            refreshPrivateSSHKeyToggle();
            continue;
        }
        element.value = nextValue;
    }
}

async function applyGeneratedSecretDefaults() {
    const result = await GenerateSecretDefaults();
    if (!result.success) {
        throw new Error(result.message || 'Не удалось подготовить безопасные секреты.');
    }
    const generatedValues = result.values || {};
    generatedSecretDefaultsPending = Object.keys(generatedValues).length > 0;
    for (const [key, value] of Object.entries(generatedValues)) {
        const fieldId = SECRETS_FIELD_MAP[key];
        if (!fieldId) continue;
        const element = document.getElementById(fieldId);
        if (!element) continue;

        if (key === 'ssh_private_key') {
            if (!privateSSHKeyValue.trim() || isTemplatePlaceholder(privateSSHKeyValue)) {
                privateSSHKeyValue = String(value ?? '');
                element.value = privateSSHKeyVisible ? privateSSHKeyValue : '';
                element.placeholder = privateSSHKeyVisible
                    ? ''
                    : (privateSSHKeyValue.trim() ? PRIVATE_SSH_KEY_HIDDEN_PLACEHOLDER : PRIVATE_SSH_KEY_EMPTY_PLACEHOLDER);
                refreshPrivateSSHKeyToggle();
            }
            continue;
        }

        if (!String(element.value ?? '').trim() || isTemplatePlaceholder(element.value)) {
            element.value = String(value ?? '');
        }
    }
}

function collectSecretFormValues(normalized = {}) {
    const values = {};
    for (const [key, fieldId] of Object.entries(SECRETS_FIELD_MAP)) {
        const element = document.getElementById(fieldId);
        if (!element) continue;
        if (key === 'ssh_private_key' && !Object.prototype.hasOwnProperty.call(normalized, key)) {
            values[key] = privateSSHKeyVisible ? String(element.value ?? '') : privateSSHKeyValue;
            continue;
        }
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
    let retainedBlankSecret = false;
    for (const [key, value] of Object.entries(values)) {
        const current = String(value ?? '');
        const existed = Object.prototype.hasOwnProperty.call(sourceSecretValues, key);
        const previous = existed ? String(sourceSecretValues[key] ?? '') : '';
        if (existed && current === previous) continue;

        // Clearing a populated credential must not silently erase it from an
        // existing secrets.yml. Enter a replacement to change the credential.
        if (SENSITIVE_SECRET_FIELDS.has(key) && existed && !sourceSecretIsTemplate &&
            current.trim() === '' && previous.trim() !== '' && !isTemplatePlaceholder(previous)) {
            retainedBlankSecret = true;
            continue;
        }
        if (!existed && current === '') continue;
        updates[key] = current;
    }
    if (retainedBlankSecret) {
        alert('Пустое поле не удаляет уже сохранённый секрет: прежнее значение будет сохранено. Чтобы заменить секрет, введите новое значение.');
    }
    return updates;
}

window.editExistingSecrets = async function() {
    if (secretFormInitialized) {
        goToStep(4);
        return;
    }
    try {
        const result = await LoadSecrets();
        if (result.success) {
            populateSecretsForm(result.values, result.isTemplate);
            await applyGeneratedSecretDefaults();
            secretFormInitialized = true;
            const selected = getSelectedProjectVersion();
            secretFormProjectSHA = selected.valid ? selected.expectedSHA : '';
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
        if (borgPass.length < 32 || !borgPass.trim()) {
            alert('Фраза шифрования Borg должна содержать не менее 32 символов.');
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
        if (s3WorkloadPrefix && !safePrefix.test(s3WorkloadPrefix)) {
            alert('Префикс Workload должен соответствовать шаблону [A-Za-z0-9][A-Za-z0-9_-]*.');
            backupFields.workloadPrefix.focus();
            return;
        }
        if (s3EdgePrefix && !safePrefix.test(s3EdgePrefix)) {
            alert('Префикс Edge должен соответствовать шаблону [A-Za-z0-9][A-Za-z0-9_-]*.');
            backupFields.edgePrefix.focus();
            return;
        }
        const safeBucket = /^[a-z0-9][a-z0-9.-]{1,61}[a-z0-9]$/;
        if (!safeBucket.test(s3Bucket) || s3Bucket.includes('..')) {
            alert('Укажите корректное S3-имя бакета (латиница в нижнем регистре, цифры, точки и дефисы; без пути).');
            backupFields.bucket.focus();
            return;
        }
    }

    if (hermesApiBaseUrl) {
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
    }
    if (hermesApiKey && (hermesApiKey.includes('\n') || hermesApiKey.includes('\r'))) {
        alert('API-ключ Hermes не должен содержать переносы строк.');
        document.getElementById('sec_hermes_api_key').focus();
        return;
    }
    if (hermesModel && (hermesModel.includes('\n') || hermesModel.includes('\r'))) {
        alert('Идентификатор модели Hermes не должен содержать переносы строк.');
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

    if (!window.confirm('Предпросмотр secrets.yml будет содержать пароли, токены, API-ключи и приватный SSH-ключ в открытом виде. Продолжайте только на доверенном устройстве. Показать конфигурацию?')) {
        return;
    }

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
        generatedSecretUpdates = { ...updates };
        generatedPreviewValues = { ...(preview.values || {}) };
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
        const result = await SaveSecrets(generatedSecretUpdates, generatedYamlString);
        if (result.success) {
            if (Object.keys(generatedPreviewValues).length > 0) {
                sourceSecretValues = { ...generatedPreviewValues };
            }
            sourceSecretIsTemplate = false;
            if (Object.prototype.hasOwnProperty.call(sourceSecretValues, 'ssh_private_key')) {
                privateSSHKeyValue = String(sourceSecretValues.ssh_private_key ?? '');
            }
            privateSSHKeyVisible = false;
            const privateKeyField = document.getElementById('sec_private_ssh_key');
            if (privateKeyField) {
                privateKeyField.value = '';
                privateKeyField.placeholder = privateSSHKeyValue.trim()
                    ? PRIVATE_SSH_KEY_HIDDEN_PLACEHOLDER
                    : PRIVATE_SSH_KEY_EMPTY_PLACEHOLDER;
            }
            refreshPrivateSSHKeyToggle();
            generatedSecretUpdates = {};
            generatedPreviewValues = {};
            generatedSecretDefaultsPending = false;
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

window.confirmDeployment = async function() {
    if (selectedOptionNum === 0) return;
    const nextButton = document.getElementById('btn-deploy-next');
    if (nextButton) {
        nextButton.disabled = true;
        nextButton.innerText = 'Проверка конфигурации...';
    }
    try {
        const validation = await ValidateDeploymentSecrets(generatedSecretUpdates, selectedOptionNum);
        if (!validation.success) {
            alert('Развёртывание заблокировано: ' + validation.message);
            return;
        }
    } catch (err) {
        alert('Не удалось проверить конфигурацию перед запуском Ansible: ' + err);
        return;
    } finally {
        if (nextButton) {
            nextButton.disabled = selectedOptionNum === 0;
            nextButton.innerText = 'Продолжить';
        }
    }

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
    btnRun.innerText = 'Деплой выполняется…';
    btnBack.disabled = true;
    if (btnExit) btnExit.style.display = 'none';
    if (btnEnter) btnEnter.style.display = 'none';

    const postDeployGuide = document.getElementById('post-deploy-guide');
    if (postDeployGuide) postDeployGuide.style.display = 'none';

    function addLog(message) {
        const logPre = document.getElementById('deploy-logs');
        const isAtBottom = (logPre.scrollHeight - logPre.clientHeight) <= (logPre.scrollTop + 5);
        logPre.innerText += message;
        // Show manual continuation only when streamed Ansible output asks for input.
        if (btnEnter && /(press\s+(?:enter|return)\s+to\s+continue|press\s+enter|нажмите\s+(?:enter|ввод)|нажмите\s+enter\s+для\s+продолжения)/i.test(logPre.innerText.slice(-4000))) {
            btnEnter.style.display = 'block';
        }
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
            logPre.style.borderColor = 'var(--success)';
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
            logPre.style.borderColor = 'var(--danger)';
            btnRun.disabled = false;
            btnRun.innerText = 'Повторить деплой';
            btnBack.disabled = false;
            if (btnExit) btnExit.style.display = 'block';
        }
    } catch (err) {
        if (btnEnter) btnEnter.style.display = 'none';
        logPre.innerText += `\n[ КРИТИЧЕСКАЯ ОШИБКА СИСТЕМЫ ]: Ошибка вызова бэкенда:\n${err}\n`;
        logPre.style.borderColor = 'var(--danger)';

        btnRun.disabled = false;
        btnRun.innerText = 'Повторить деплой';
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
let projectInstallDirectory = "";
let projectInstallDirectoryVerified = false;
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

function selectedProjectAction() {
    return document.querySelector('input[name="project-action"]:checked')?.value || 'install';
}

function updateProjectDirectoryStatus(message, isWarning = false) {
    const status = document.getElementById('project-install-directory-status');
    if (!status) return;
    status.textContent = message;
    status.classList.toggle('is-warning', Boolean(isWarning));
}

window.markProjectInstallDirectoryChanged = function() {
    projectInstallDirectoryVerified = false;
    projectInstallState = null;
    updateProjectDirectoryStatus('Путь изменён. Нажмите «Проверить каталог» перед продолжением.', true);
    window.updateProjectActionUI();
};

window.verifyProjectInstallDirectory = async function() {
    const input = document.getElementById('project-install-directory');
    try {
        const result = await InspectProjectInstallDirectory(String(input?.value ?? '').trim());
        if (!result.success) {
            projectInstallDirectoryVerified = false;
            projectInstallState = null;
            updateProjectDirectoryStatus(result.message || 'Каталог не прошёл проверку.', true);
            window.updateProjectActionUI();
            return;
        }
        projectInstallDirectory = result.directory;
        projectInstallState = result.installed;
        projectInstallDirectoryVerified = true;
        updateProjectDirectoryStatus(result.message || ('Каталог проверен: ' + result.directory), Boolean(result.installed?.partial));
        renderInstalledProjectState(projectInstallState, false);
        window.updateProjectActionUI();
    } catch (error) {
        projectInstallDirectoryVerified = false;
        updateProjectDirectoryStatus('Не удалось проверить каталог: ' + String(error), true);
        window.updateProjectActionUI();
    }
};

window.chooseProjectInstallDirectory = async function() {
    const button = document.getElementById('btn-choose-install-directory');
    if (button) button.disabled = true;
    try {
        const result = await ChooseProjectInstallDirectory();
        if (!result.success) {
            if (result.message && result.message !== 'Выбор каталога отменён.') updateProjectDirectoryStatus(result.message, true);
            return;
        }
        projectInstallDirectory = result.directory;
        projectInstallState = result.installed;
        projectInstallDirectoryVerified = true;
        const input = document.getElementById('project-install-directory');
        if (input) input.value = result.directory;
        updateProjectDirectoryStatus(result.message || ('Каталог: ' + result.directory), Boolean(result.installed?.partial));
        renderInstalledProjectState(projectInstallState, true);
        window.updateProjectActionUI();
    } catch (error) {
        updateProjectDirectoryStatus('Не удалось выбрать каталог: ' + String(error), true);
    } finally {
        if (button) button.disabled = false;
    }
};

window.updateProjectActionUI = function() {
    const action = selectedProjectAction();
    const state = projectInstallState;
    const select = document.getElementById('project-version-select');
    const useOption = document.querySelector('input[name="project-action"][value="use"]');
    const installOption = document.querySelector('input[name="project-action"][value="install"]');
    if (useOption) useOption.disabled = !(state && state.exists && state.complete && state.knownVersion && state.integrityVerified);
    if (installOption) installOption.disabled = !projectInstallDirectoryVerified;
    if (select) select.disabled = !projectVersionsLoaded || action === 'use';
    const button = document.getElementById('btn-version-continue');
    if (button) button.textContent = action === 'use' ? 'Использовать установленный репозиторий' : 'Установить выбранный тег';
    window.updateProjectVersionPreview();
};

function setProjectVersionContinueEnabled(versionEnabled) {
    const button = document.getElementById('btn-version-continue');
    if (!button) return;
    const action = selectedProjectAction();
    const state = projectInstallState;
    const pathReady = projectInstallDirectoryVerified && Boolean(document.getElementById('project-install-directory')?.value.trim());
    const useReady = Boolean(state && state.exists && state.complete && state.knownVersion && state.integrityVerified);
    button.disabled = !pathReady || (action === 'use' ? !useReady : !(projectVersionsLoaded && versionEnabled));
}

function getSelectedProjectVersion() {
    if (!projectVersionsLoaded) return { valid: false, message: 'Список версий ещё не загружен. Обновите список и повторите.' };
    const option = selectedVersionOption();
    if (!option || !option.dataset.ref || !isFullSHA(option.dataset.sha)) {
        return { valid: false, message: 'Выберите тег или ветку dev с подтверждённым commit SHA.' };
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

    if (result.dev && result.dev.ref === 'dev' && isFullSHA(result.dev.commitSHA)) {
        const devGroup = document.createElement('optgroup');
        devGroup.label = 'Ветка';
        const devOption = document.createElement('option');
        devOption.value = 'dev';
        devOption.textContent = 'dev — последние изменения (нестабильно)';
        devOption.dataset.ref = 'dev';
        devOption.dataset.sha = result.dev.commitSHA;
        devOption.dataset.mode = 'dev';
        devGroup.appendChild(devOption);
        select.appendChild(devGroup);
    }

    if (Array.isArray(result.tags) && result.tags.length) {
        const group = document.createElement('optgroup');
        group.label = 'Опубликованные теги';
        for (const tag of result.tags) {
            const option = document.createElement('option');
            option.value = 'tag:' + tag.ref;
            option.textContent = tag.ref;
            option.dataset.ref = tag.ref;
            option.dataset.sha = tag.commitSHA;
            option.dataset.mode = 'tag';
            group.appendChild(option);
        }
        select.appendChild(group);
    }

    const values = Array.from(select.options).map(option => option.value);
    if (preserveChoice && values.includes(previousValue)) select.value = previousValue;
    else if (select.options.length) select.selectedIndex = 0;
    select.disabled = !projectVersionsLoaded || selectedProjectAction() === 'use';
}

function renderInstalledProjectState(state, initialLoad) {
    const note = document.getElementById('project-installed-note');
    if (!note) return;

    const currentAction = selectedProjectAction();
    const canUse = Boolean(state && state.exists && state.complete && state.knownVersion && state.integrityVerified);
    if (initialLoad) {
        const installRadio = document.querySelector('input[name="project-action"][value="install"]');
        if (installRadio) installRadio.checked = true;
    } else if (currentAction === 'use' && !canUse) {
        const installRadio = document.querySelector('input[name="project-action"][value="install"]');
        if (installRadio) installRadio.checked = true;
    }
    if (state && state.exists && state.partial) {
        note.style.display = 'block';
        note.classList.add('is-warning');
        note.textContent = 'Найдена неполная установка: ' + (state.message || 'структура не прошла проверку') + '. Для безопасности выберите другой каталог или устраните проблему вручную.';
    } else if (state && state.exists && !canUse) {
        note.style.display = 'block';
        note.classList.add('is-warning');
        note.textContent = (state.message || 'Версия установленного репозитория не подтверждена.') + ' Стандартная установка сохранит прежний каталог в резервной копии перед заменой.';
    } else if (canUse) {
        note.style.display = 'block';
        note.classList.remove('is-warning');
        note.textContent = 'В каталоге установлено: ' + state.metadata.display_name + ' (commit ' + state.metadata.commit_sha + '). Можно использовать эту установку или выполнить стандартную установку выбранного тега.';
    } else {
        note.style.display = 'none';
        note.textContent = '';
        note.classList.remove('is-warning');
    }
    const pathStatus = state && state.directory
        ? ('Каталог: ' + state.directory + '. ' + (state.message || ''))
        : (state?.message || '');
    if (projectInstallDirectoryVerified && pathStatus) updateProjectDirectoryStatus(pathStatus, Boolean(state?.partial));
    window.updateProjectActionUI();
}

window.initializeProjectVersions = async function(preserveChoice = false) {
    const select = document.getElementById('project-version-select');
    const summary = document.getElementById('project-version-summary');
    const continueButton = document.getElementById('btn-version-continue');
    const refreshButton = document.getElementById('btn-refresh-project-versions');
    if (refreshButton) refreshButton.disabled = true;
    if (continueButton) continueButton.disabled = true;
    if (!preserveChoice && summary) summary.textContent = 'Получаем опубликованные теги с GitHub...';

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
        if (select) select.disabled = true;
        if (summary) summary.textContent = 'Не удалось получить список версий: ' + String(error) +
            '. Использование проверенной текущей установки и восстановление не требуют доступа к GitHub.';
        window.updateProjectActionUI();
        window.updateProjectVersionPreview();
    } finally {
        if (refreshButton) refreshButton.disabled = false;
    }
};

window.refreshProjectVersions = async function() {
    await window.initializeProjectVersions(true);
};

window.updateProjectVersionPreview = function() {
    const summary = document.getElementById('project-version-summary');
    const warning = document.getElementById('project-version-warning');
    if (!summary || !warning) return;
    if (selectedProjectAction() === 'use') {
        summary.textContent = projectInstallState?.metadata?.commit_sha
            ? 'Будет использована проверенная текущая версия ' + projectInstallState.metadata.display_name + '.'
            : 'Нет полной установки с подтверждённой версией.';
        warning.textContent = 'Текущий репозиторий не скачивается и не изменяется.';
        setProjectVersionContinueEnabled(true);
        return;
    }

    const option = selectedVersionOption();
    if (!option || !option.dataset.ref || !isFullSHA(option.dataset.sha)) {
        summary.textContent = 'Выберите опубликованный тег или ветку dev.';
        setProjectVersionContinueEnabled(false);
        return;
    }

    if (option.value === 'dev') {
        summary.textContent = 'Будет установлена ветка dev на текущем commit ' + option.dataset.sha + '.';
        warning.textContent = 'Ветка dev содержит последние изменения и может быть нестабильной. Установщик закрепит показанный commit; если ветка изменится до загрузки, установка остановится и потребует обновить список версий.';
    } else if (option.value.startsWith('tag:')) {
        summary.textContent = 'Будет установлен тег ' + option.dataset.ref + ' (commit ' + option.dataset.sha + ').';
        warning.textContent = projectInstallState?.exists
            ? 'Текущий каталог будет заменён только после загрузки и проверки нового кода. Прежний каталог сохранится в резервной копии; данные контейнеров и БД не откатываются.'
            : 'Тег будет загружен во временный каталог и проверен до установки.';
    } else {
        summary.textContent = 'Выберите поддерживаемый тег или ветку dev.';
        setProjectVersionContinueEnabled(false);
        return;
    }
    setProjectVersionContinueEnabled(true);
};

window.continueFromVersionSelection = async function() {
    if (!projectInstallDirectoryVerified) {
        updateProjectDirectoryStatus('Сначала проверьте каталог установки.', true);
        return;
    }
    if (selectedProjectAction() === 'use') {
        const state = projectInstallState;
        if (!(state && state.exists && state.complete && state.knownVersion && state.integrityVerified)) {
            const summary = document.getElementById('project-version-summary');
            if (summary) summary.textContent = 'В выбранном каталоге нет полной установки с проверенной версией. Выберите стандартную установку.';
            return;
        }
        window.goToStep(2);
        return;
    }
    const selection = getSelectedProjectVersion();
    if (!selection.valid) {
        const summary = document.getElementById('project-version-summary');
        if (summary) summary.textContent = selection.message;
        return;
    }
    window.goToStep(2);
};

window.initializeProjectInstallDirectory = async function() {
    try {
        const result = await GetProjectInstallDirectory();
        if (!result.success) {
            projectInstallDirectoryVerified = false;
            updateProjectDirectoryStatus(result.message || 'Не удалось определить каталог установки.', true);
            return;
        }
        projectInstallDirectory = result.directory;
        projectInstallState = result.installed;
        projectInstallDirectoryVerified = true;
        const input = document.getElementById('project-install-directory');
        if (input) input.value = projectInstallDirectory;
        updateProjectDirectoryStatus(result.message || ('Каталог установки: ' + projectInstallDirectory), Boolean(result.installed?.partial));
        renderInstalledProjectState(projectInstallState, true);
    } catch (error) {
        projectInstallDirectoryVerified = false;
        updateProjectDirectoryStatus('Не удалось определить каталог установки: ' + String(error), true);
    }
};

window.cancelProjectInstallation = async function() {
    const button = document.getElementById('btn-install-cancel');
    if (button) button.disabled = true;
    try {
        const result = await CancelProjectInstallation();
        const status = document.getElementById('install-text');
        if (status && result.message) status.innerText = result.message;
    } catch (error) {
        const status = document.getElementById('install-text');
        if (status) status.innerText = 'Не удалось отменить загрузку: ' + String(error);
    } finally {
        if (button) button.disabled = false;
    }
};

setupSecretVisibilityToggles();
updateStepProgress(0);
window.initializeProjectInstallDirectory().then(() => window.initializeProjectVersions());