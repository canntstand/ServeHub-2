import './style.css';
import { CheckDocker, GetProjectVersions, ResolveProjectVersion, GetProjectInstallDirectory, InspectProjectInstallDirectory, SetProjectInstallDirectory, ChooseProjectInstallDirectory, PerformProjectAction, CancelProjectInstallation, SaveSecrets, PreviewSecrets, ValidateDeploymentSecrets, CloseApp, MinimizeApp, MaximizeApp, RunDeployment, CheckSecrets, LoadSecrets, SendEnter } from '../wailsjs/go/main/App.js';
import { EventsOn } from '../wailsjs/runtime/runtime.js';

let generatedYamlString = "";
let generatedSecretUpdates = {};
let generatedPreviewValues = {};
let sourceSecretValues = {};
let sourceSecretIsTemplate = true;
let secretFormInitialized = false;
let secretFormProjectSHA = "";
let secretFormProjectDirectory = "";

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
            const selectedCommitSHA = selection?.expectedSHA ?? String(projectInstallState?.metadata?.commit_sha ?? '');
            if (!secretFormInitialized || secretFormProjectSHA !== selectedCommitSHA ||
                secretFormProjectDirectory !== projectInstallDirectory) {
                populateSecretsForm(existingSecrets.values, existingSecrets.isTemplate);
                secretFormInitialized = true;
                secretFormProjectSHA = selectedCommitSHA;
                secretFormProjectDirectory = projectInstallDirectory;
            }

            spinner.style.display = 'none';

            const existingPepper = String(existingSecrets.values?.hbox_pepper ?? '').trim();
            const generatedPepper = String(document.getElementById('sec_hbox_pepper')?.value ?? '').trim();
            const existingHermesDashboardSecret = String(existingSecrets.values?.hermes_dashboard_basic_auth_secret ?? '').trim();
            const generatedHermesDashboardSecret = String(document.getElementById('sec_hermes_dashboard_basic_auth_secret')?.value ?? '').trim();
            const generatedSecuritySecretsNeedSaving = !existingSecrets.isTemplate && (
                existingPepper !== generatedPepper ||
                existingHermesDashboardSecret !== generatedHermesDashboardSecret
            );

            if (secretsCheck.success && !existingSecrets.isTemplate && !generatedSecuritySecretsNeedSaving) {
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
                installText.innerText = generatedSecuritySecretsNeedSaving
                    ? `✓ ${result.message}\n\nУстановщик подготовил новые секреты Homebox/Hermes. Перейдите к форме, чтобы проверить и сохранить их перед деплоем.`
                    : `✓ ${result.message}`;
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
    postgres_user: 'sec_pg_user',
    hbox_pepper: 'sec_hbox_pepper',
    admin_password: 'sec_admin_pass',
    email: 'sec_email',
    telegram_token: 'sec_tg_token',
    telegram_chat_id: 'sec_tg_chat_id',
    webnames_apikey: 'sec_webnames',
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

const BOOTSTRAP_AUTH_FIELDS = {
    vps: { root: 'vps_root_password', user: ['vps_ssh_password', 'vps_sudo_password'] },
    local: { root: 'local_root_password', user: ['local_ssh_password', 'local_sudo_password'] },
};
let bootstrapAuthModes = { vps: 'root', local: 'root' };
let separateSudoPasswords = { vps: false, local: false };

function inferSeparateSudoPassword(values, server) {
    const [sshKey, sudoKey] = BOOTSTRAP_AUTH_FIELDS[server].user;
    const sshValue = String(values?.[sshKey] ?? '').trim();
    const sudoValue = String(values?.[sudoKey] ?? '').trim();
    return sudoValue !== '' && sudoValue !== sshValue;
}

function applySeparateSudoPassword(server, enabled) {
    if (!Object.prototype.hasOwnProperty.call(BOOTSTRAP_AUTH_FIELDS, server)) return;
    separateSudoPasswords[server] = Boolean(enabled);

    const checkbox = document.getElementById(`sec_${server}_separate_sudo`);
    if (checkbox) checkbox.checked = separateSudoPasswords[server];

    const sshLabel = document.getElementById(`sec_${server}_ssh_password_label`);
    if (sshLabel) {
        sshLabel.textContent = separateSudoPasswords[server]
            ? 'SSH-пароль пользователя:'
            : 'Пароль пользователя для SSH и sudo:';
    }
    const sshInput = document.getElementById(`sec_${server}_ssh_pass`);
    if (sshInput && sshLabel) sshInput.placeholder = sshLabel.textContent;

    const sudoField = document.querySelector(`[data-separate-sudo-field="${server}"]`);
    if (sudoField) {
        sudoField.hidden = bootstrapAuthModes[server] !== 'user' || !separateSudoPasswords[server];
    }
}

function addInactiveBootstrapCredentialRemovals(updates) {
    for (const [server, fields] of Object.entries(BOOTSTRAP_AUTH_FIELDS)) {
        const mode = bootstrapAuthModes[server] === 'user' ? 'user' : 'root';
        const inactiveKeys = mode === 'root' ? fields.user : [fields.root];
        for (const key of inactiveKeys) {
            updates[`__delete_auth__${key}`] = 'true';
        }
    }
    return updates;
}

function inferBootstrapAuthMode(values, server) {
    const fields = BOOTSTRAP_AUTH_FIELDS[server];
    const rootValue = String(values?.[fields.root] ?? '').trim();
    const userValues = fields.user.map(key => String(values?.[key] ?? '').trim());
    if (rootValue !== '' && !isTemplatePlaceholder(rootValue)) return 'root';
    if (userValues.some(value => value !== '' && !isTemplatePlaceholder(value))) return 'user';
    return 'root';
}

function applyBootstrapAuthMode(server, mode) {
    if (!Object.prototype.hasOwnProperty.call(BOOTSTRAP_AUTH_FIELDS, server)) return;
    const selectedMode = mode === 'user' ? 'user' : 'root';
    bootstrapAuthModes[server] = selectedMode;
    const selector = document.getElementById(`sec_${server}_auth_mode`);
    if (selector && selector.value !== selectedMode) selector.value = selectedMode;

    document.querySelectorAll(`[data-bootstrap-auth^="${server}:"]`).forEach((field) => {
        const fieldMode = String(field.getAttribute('data-bootstrap-auth') || '').split(':')[1];
        field.hidden = fieldMode !== selectedMode;
    });
    applySeparateSudoPassword(server, separateSudoPasswords[server]);
}

window.setBootstrapAuthMode = function(server, mode) {
    applyBootstrapAuthMode(server, mode);
};

window.setSeparateSudoPassword = function(server, enabled) {
    applySeparateSudoPassword(server, enabled);
};

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

function generateSecureHexSecret(byteLength) {
    if (!window.crypto || typeof window.crypto.getRandomValues !== 'function') return '';
    try {
        const secretBytes = new Uint8Array(byteLength);
        window.crypto.getRandomValues(secretBytes);
        return Array.from(secretBytes, byte => byte.toString(16).padStart(2, '0')).join('');
    } catch (_) {
        return '';
    }
}

function utf8ByteLength(value) {
    const text = String(value ?? '');
    if (typeof TextEncoder !== 'undefined') return new TextEncoder().encode(text).length;
    return new Blob([text]).size;
}

function isHexSecretOfLength(value, byteLength) {
    return new RegExp(`^[0-9a-fA-F]{${byteLength * 2}}import './style.css';
import { CheckDocker, GetProjectVersions, ResolveProjectVersion, GetProjectInstallDirectory, InspectProjectInstallDirectory, SetProjectInstallDirectory, ChooseProjectInstallDirectory, PerformProjectAction, CancelProjectInstallation, SaveSecrets, PreviewSecrets, ValidateDeploymentSecrets, CloseApp, MinimizeApp, MaximizeApp, RunDeployment, CheckSecrets, LoadSecrets, SendEnter } from '../wailsjs/go/main/App.js';
import { EventsOn } from '../wailsjs/runtime/runtime.js';

let generatedYamlString = "";
let generatedSecretUpdates = {};
let generatedPreviewValues = {};
let sourceSecretValues = {};
let sourceSecretIsTemplate = true;
let secretFormInitialized = false;
let secretFormProjectSHA = "";
let secretFormProjectDirectory = "";

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
            const selectedCommitSHA = selection?.expectedSHA ?? String(projectInstallState?.metadata?.commit_sha ?? '');
            if (!secretFormInitialized || secretFormProjectSHA !== selectedCommitSHA ||
                secretFormProjectDirectory !== projectInstallDirectory) {
                populateSecretsForm(existingSecrets.values, existingSecrets.isTemplate);
                secretFormInitialized = true;
                secretFormProjectSHA = selectedCommitSHA;
                secretFormProjectDirectory = projectInstallDirectory;
            }

            spinner.style.display = 'none';

            const existingPepper = String(existingSecrets.values?.hbox_pepper ?? '').trim();
            const generatedPepper = String(document.getElementById('sec_hbox_pepper')?.value ?? '').trim();
            const existingHermesDashboardSecret = String(existingSecrets.values?.hermes_dashboard_basic_auth_secret ?? '').trim();
            const generatedHermesDashboardSecret = String(document.getElementById('sec_hermes_dashboard_basic_auth_secret')?.value ?? '').trim();
            const generatedSecuritySecretsNeedSaving = !existingSecrets.isTemplate && (
                existingPepper !== generatedPepper ||
                existingHermesDashboardSecret !== generatedHermesDashboardSecret
            );

            if (secretsCheck.success && !existingSecrets.isTemplate && !generatedSecuritySecretsNeedSaving) {
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
                installText.innerText = generatedSecuritySecretsNeedSaving
                    ? `✓ ${result.message}\n\nУстановщик подготовил новые секреты Homebox/Hermes. Перейдите к форме, чтобы проверить и сохранить их перед деплоем.`
                    : `✓ ${result.message}`;
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
    postgres_user: 'sec_pg_user',
    hbox_pepper: 'sec_hbox_pepper',
    admin_password: 'sec_admin_pass',
    email: 'sec_email',
    telegram_token: 'sec_tg_token',
    telegram_chat_id: 'sec_tg_chat_id',
    webnames_apikey: 'sec_webnames',
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

const BOOTSTRAP_AUTH_FIELDS = {
    vps: { root: 'vps_root_password', user: ['vps_ssh_password', 'vps_sudo_password'] },
    local: { root: 'local_root_password', user: ['local_ssh_password', 'local_sudo_password'] },
};
let bootstrapAuthModes = { vps: 'root', local: 'root' };
let separateSudoPasswords = { vps: false, local: false };

function inferSeparateSudoPassword(values, server) {
    const [sshKey, sudoKey] = BOOTSTRAP_AUTH_FIELDS[server].user;
    const sshValue = String(values?.[sshKey] ?? '').trim();
    const sudoValue = String(values?.[sudoKey] ?? '').trim();
    return sudoValue !== '' && sudoValue !== sshValue;
}

function applySeparateSudoPassword(server, enabled) {
    if (!Object.prototype.hasOwnProperty.call(BOOTSTRAP_AUTH_FIELDS, server)) return;
    separateSudoPasswords[server] = Boolean(enabled);

    const checkbox = document.getElementById(`sec_${server}_separate_sudo`);
    if (checkbox) checkbox.checked = separateSudoPasswords[server];

    const sshLabel = document.getElementById(`sec_${server}_ssh_password_label`);
    if (sshLabel) {
        sshLabel.textContent = separateSudoPasswords[server]
            ? 'SSH-пароль пользователя:'
            : 'Пароль пользователя для SSH и sudo:';
    }
    const sshInput = document.getElementById(`sec_${server}_ssh_pass`);
    if (sshInput && sshLabel) sshInput.placeholder = sshLabel.textContent;

    const sudoField = document.querySelector(`[data-separate-sudo-field="${server}"]`);
    if (sudoField) {
        sudoField.hidden = bootstrapAuthModes[server] !== 'user' || !separateSudoPasswords[server];
    }
}

function addInactiveBootstrapCredentialRemovals(updates) {
    for (const [server, fields] of Object.entries(BOOTSTRAP_AUTH_FIELDS)) {
        const mode = bootstrapAuthModes[server] === 'user' ? 'user' : 'root';
        const inactiveKeys = mode === 'root' ? fields.user : [fields.root];
        for (const key of inactiveKeys) {
            updates[`__delete_auth__${key}`] = 'true';
        }
    }
    return updates;
}

function inferBootstrapAuthMode(values, server) {
    const fields = BOOTSTRAP_AUTH_FIELDS[server];
    const rootValue = String(values?.[fields.root] ?? '').trim();
    const userValues = fields.user.map(key => String(values?.[key] ?? '').trim());
    if (rootValue !== '' && !isTemplatePlaceholder(rootValue)) return 'root';
    if (userValues.some(value => value !== '' && !isTemplatePlaceholder(value))) return 'user';
    return 'root';
}

function applyBootstrapAuthMode(server, mode) {
    if (!Object.prototype.hasOwnProperty.call(BOOTSTRAP_AUTH_FIELDS, server)) return;
    const selectedMode = mode === 'user' ? 'user' : 'root';
    bootstrapAuthModes[server] = selectedMode;
    const selector = document.getElementById(`sec_${server}_auth_mode`);
    if (selector && selector.value !== selectedMode) selector.value = selectedMode;

    document.querySelectorAll(`[data-bootstrap-auth^="${server}:"]`).forEach((field) => {
        const fieldMode = String(field.getAttribute('data-bootstrap-auth') || '').split(':')[1];
        field.hidden = fieldMode !== selectedMode;
    });
    applySeparateSudoPassword(server, separateSudoPasswords[server]);
}

window.setBootstrapAuthMode = function(server, mode) {
    applyBootstrapAuthMode(server, mode);
};

window.setSeparateSudoPassword = function(server, enabled) {
    applySeparateSudoPassword(server, enabled);
};

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

).test(String(value ?? '').trim());
}

function initializeGeneratedSecuritySecrets() {
    const hboxField = document.getElementById('sec_hbox_pepper');
    const hermesField = document.getElementById('sec_hermes_dashboard_basic_auth_secret');
    if (!hboxField || !hermesField) {
        alert('Не удалось подготовить секреты Homebox/Hermes: поля конфигурации не найдены.');
        return false;
    }

    const currentPepper = String(hboxField.value ?? '').trim();
    if (utf8ByteLength(currentPepper) < 32) {
        const generatedPepper = generateSecureHexSecret(32);
        if (!generatedPepper) {
            alert('Не удалось безопасно сгенерировать ключ Homebox. Проверьте поддержку криптографического генератора в установщике.');
            return false;
        }
        hboxField.value = generatedPepper;
    }

    const currentHermesSecret = String(hermesField.value ?? '').trim();
    if (!isHexSecretOfLength(currentHermesSecret, 32)) {
        const generatedHermesSecret = generateSecureHexSecret(32);
        if (!generatedHermesSecret) {
            alert('Не удалось безопасно сгенерировать секрет Hermes Dashboard. Проверьте поддержку криптографического генератора в установщике.');
            return false;
        }
        hermesField.value = generatedHermesSecret;
    }

    return true;
}

function populateSecretsForm(values, isTemplate = false) {
    const source = values && typeof values === 'object' ? values : {};
    sourceSecretValues = { ...source };
    sourceSecretIsTemplate = Boolean(isTemplate);

    for (const [key, fieldId] of Object.entries(SECRETS_FIELD_MAP)) {
        const element = document.getElementById(fieldId);
        if (!element) continue;

        let nextValue = '';
        if (!sourceSecretIsTemplate && Object.prototype.hasOwnProperty.call(source, key)) {
            nextValue = String(source[key] ?? '');
        }

        element.value = nextValue;
    }

    for (const server of Object.keys(BOOTSTRAP_AUTH_FIELDS)) {
        applyBootstrapAuthMode(server, inferBootstrapAuthMode(source, server));
        applySeparateSudoPassword(server, inferSeparateSudoPassword(source, server));
    }

    initializeGeneratedSecuritySecrets();
}

function collectSecretFormValues(normalized = {}) {
    const values = {};
    for (const [key, fieldId] of Object.entries(SECRETS_FIELD_MAP)) {
        const element = document.getElementById(fieldId);
        if (!element) continue;
        const [server, credential] = (() => {
            if (key.startsWith('vps_')) return ['vps', key];
            if (key.startsWith('local_')) return ['local', key];
            return ['', key];
        })();
        const authFields = BOOTSTRAP_AUTH_FIELDS[server];
        if (authFields && key === authFields.user[1] &&
            bootstrapAuthModes[server] === 'user' && !separateSudoPasswords[server]) {
            const sshField = document.getElementById(SECRETS_FIELD_MAP[authFields.user[0]]);
            values[key] = String(sshField?.value ?? '');
            continue;
        }

        const credentialGroup = element.closest('.bootstrap-auth-field');
        if (credentialGroup?.hidden) continue;
        values[key] = Object.prototype.hasOwnProperty.call(normalized, key)
            ? normalized[key]
            : String(element.value ?? '');
    }
    return values;
}

function buildChangedSecretUpdates(values) {
    const updates = {};
    let retainedBlankSecret = false;
    for (const [key, value] of Object.entries(values)) {
        const current = String(value ?? '');
        const existed = Object.prototype.hasOwnProperty.call(sourceSecretValues, key);
        const previous = existed ? String(sourceSecretValues[key] ?? '') : '';
        if (existed && current === previous) continue;

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
    const hboxField = document.getElementById('sec_hbox_pepper');
    const hboxPepper = String(hboxField?.value ?? '').trim();
    if (utf8ByteLength(hboxPepper) < 32) {
        alert('Ключ Homebox должен содержать не менее 32 байт. Оставьте автоматически созданный ключ или введите более длинное значение.');
        goToStep(4);
        hboxField?.focus();
        return;
    }

    const hermesSecretField = document.getElementById('sec_hermes_dashboard_basic_auth_secret');
    const hermesDashboardSecret = String(hermesSecretField?.value ?? '').trim();
    if (!isHexSecretOfLength(hermesDashboardSecret, 32)) {
        alert('Не удалось подготовить внутренний секрет Hermes Dashboard. Повторно откройте конфигурацию и попробуйте ещё раз.');
        goToStep(4);
        return;
    }

    const normalizedValues = collectSecretFormValues({
        hbox_pepper: hboxPepper,
        hermes_dashboard_basic_auth_secret: hermesDashboardSecret,
    });
    const updates = addInactiveBootstrapCredentialRemovals(buildChangedSecretUpdates(normalizedValues));

    const previewButton = document.querySelector('#step-4 .button-group .btn-primary');
    const originalButtonText = previewButton ? previewButton.innerText : '';
    if (previewButton) {
        previewButton.disabled = true;
        previewButton.innerText = 'Формирование YAML...';
    }

    try {
        const preview = await PreviewSecrets(updates);
        if (!preview.success) {
            alert('Не удалось сформировать secrets.yml: ' + preview.message);
            return;
        }
        const effectiveAdminPassword = String(preview.values?.admin_password ?? '');
        if (Array.from(effectiveAdminPassword).length < 12) {
            alert('ADMIN_PASSWORD должен содержать не менее 12 символов: wg-easy не принимает более короткие пароли.');
            goToStep(4);
            document.getElementById('sec_admin_pass')?.focus();
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
        alert('Не удалось сформировать конфигурацию YAML: ' + err);
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
            generatedSecretUpdates = {};
            generatedPreviewValues = {};
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

    const dependencyNote = document.getElementById('deploy-dependency-note');
    if (dependencyNote) {
        const needsExistingEdge = num === 3 || num === 4;
        dependencyNote.style.display = needsExistingEdge ? 'block' : 'none';
        dependencyNote.textContent = needsExistingEdge
            ? 'Зависимость от Edge/VPS: сценарии 3 и 4 настраивают Workload, но не развёртывают Edge. До любых изменений на Workload установщик проверит SSH-доступ к VPS и состояние контейнера wg-easy. Edge должен быть уже развёрнут, доступен по SSH, а wg-easy должен быть в состоянии healthy. Если проверка не пройдёт, развёртывание Workload не начнётся.'
            : '';
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

function validateSelectedBootstrapCredentials(option) {
    const targets = [];
    if (option === 1 || option === 5) targets.push('vps');
    if (option === 1 || option === 3) targets.push('local');

    const values = collectSecretFormValues();

    for (const server of targets) {
        const label = server === 'vps' ? 'VPS' : 'домашнего сервера';
        const selectedMode = bootstrapAuthModes[server] || 'root';
        const authFields = BOOTSTRAP_AUTH_FIELDS[server];
        const credentialKeys = selectedMode === 'root'
            ? [authFields.root]
            : separateSudoPasswords[server]
                ? authFields.user
                : [authFields.user[0]];

        for (const key of credentialKeys) {
            if (!String(values[key] ?? '').trim()) {
                const credentialName = key.endsWith('_root_password')
                    ? 'пароль root'
                    : key.endsWith('_ssh_password')
                        ? 'SSH-пароль пользователя'
                        : 'пароль sudo пользователя';
                return `${label}: выбран режим "${selectedMode === 'root' ? 'root' : 'обычный пользователь'}"; заполните поле "${credentialName}".`;
            }
        }
    }
    return '';
}

window.confirmDeployment = async function() {
    if (selectedOptionNum === 0) return;
    const nextButton = document.getElementById('btn-deploy-next');
    if (nextButton) {
        nextButton.disabled = true;
        nextButton.innerText = 'Проверка конфигурации...';
    }
    try {
        const credentialError = validateSelectedBootstrapCredentials(selectedOptionNum);
        if (credentialError) {
            alert(credentialError);
            goToStep(4);
            return;
        }
        const validation = await ValidateDeploymentSecrets(generatedSecretUpdates, selectedOptionNum);
        if (!validation.success) {
            alert('Заполните обязательные поля выбранного сценария: ' + validation.message);
            goToStep(4);
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

function isFullSHA(value) {
    return /^[a-f0-9]{40}$/i.test(String(value || '').trim());
}

function selectedVersionOption() {
    const select = document.getElementById('project-version-select');
    if (!select || !select.selectedOptions || !select.selectedOptions[0]) return null;
    return select.selectedOptions[0];
}

function selectedProjectAction() {
    const state = projectInstallState;
    if (state && state.exists && state.complete && state.knownVersion && state.integrityVerified) {
        return 'use';
    }
    return 'install';
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
    const note = document.getElementById('project-installed-note');
    if (note) {
        note.style.display = 'block';
        note.classList.remove('is-warning');
        note.textContent = 'Проверьте новый каталог, чтобы определить, установлен ли в нём ServeHub-2.';
    }
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
            const note = document.getElementById('project-installed-note');
            if (note) {
                note.style.display = 'block';
                note.classList.add('is-warning');
                note.textContent = (result.message || 'Каталог не прошёл проверку.') + ' Автоматическое определение установки не завершено.';
            }
            window.updateProjectActionUI();
            return;
        }
        projectInstallDirectory = result.directory;
        projectInstallState = result.installed;
        projectInstallDirectoryVerified = true;
        updateProjectDirectoryStatus('Каталог проверен: ' + result.directory + '.', Boolean(result.installed?.partial));
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
        updateProjectDirectoryStatus('Каталог проверен: ' + result.directory + '.', Boolean(result.installed?.partial));
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
    const reusingExisting = action === 'use';
    const select = document.getElementById('project-version-select');
    const versionLabel = document.querySelector('label[for="project-version-select"]');
    const versionSummary = document.getElementById('project-version-summary');
    const versionWarning = document.getElementById('project-version-warning');
    const versionActions = document.querySelector('.version-actions');

    if (select) {
        select.disabled = !projectVersionsLoaded || !projectInstallDirectoryVerified || reusingExisting;
        select.style.display = reusingExisting ? 'none' : '';
    }
    if (versionLabel) versionLabel.style.display = reusingExisting ? 'none' : '';
    if (versionSummary) versionSummary.style.display = reusingExisting ? 'none' : '';
    if (versionWarning) versionWarning.style.display = reusingExisting ? 'none' : '';
    if (versionActions) versionActions.style.display = reusingExisting ? 'none' : '';

    const button = document.getElementById('btn-version-continue');
    if (button) {
        button.textContent = reusingExisting
            ? 'Продолжить с установленным репозиторием'
            : 'Установить выбранную версию';
    }
    window.updateProjectVersionPreview();
};

function setProjectVersionContinueEnabled(versionEnabled) {
    const button = document.getElementById('btn-version-continue');
    if (!button) return;
    const pathReady = projectInstallDirectoryVerified && Boolean(document.getElementById('project-install-directory')?.value.trim());
    const state = projectInstallState;
    if (!pathReady || (state && state.partial)) {
        button.disabled = true;
        return;
    }
    if (selectedProjectAction() === 'use') {
        button.disabled = !(state && state.exists && state.complete && state.knownVersion && state.integrityVerified);
        return;
    }
    button.disabled = !(projectVersionsLoaded && versionEnabled);
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

    if (Array.isArray(result.tags) && result.tags.length) {
        const tagGroup = document.createElement('optgroup');
        tagGroup.label = 'Опубликованные теги';
        for (const tag of result.tags) {
            const option = document.createElement('option');
            option.value = 'tag:' + tag.ref;
            option.textContent = tag.ref;
            option.dataset.ref = tag.ref;
            option.dataset.sha = tag.commitSHA;
            option.dataset.mode = 'tag';
            tagGroup.appendChild(option);
        }
        select.appendChild(tagGroup);
    }

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

    const values = Array.from(select.options).map(option => option.value);
    if (preserveChoice && values.includes(previousValue)) select.value = previousValue;
    else if (select.options.length) select.selectedIndex = 0;
    select.disabled = !projectVersionsLoaded || selectedProjectAction() === 'use';
}

function renderInstalledProjectState(state, initialLoad) {
    const note = document.getElementById('project-installed-note');
    if (!note) return;

    const canVerifyExisting = Boolean(state && state.exists && state.complete && state.knownVersion && state.integrityVerified);
    note.style.display = 'block';
    note.classList.remove('is-warning');

    if (state && state.partial) {
        note.classList.add('is-warning');
        note.textContent = 'В выбранном каталоге обнаружена неполная установка ServeHub-2: ' +
            (state.message || 'структура проекта не прошла проверку') +
            '. Автоматическая установка заблокирована, чтобы не перезаписать непроверенные данные. Выберите другой каталог или устраните проблему вручную.';
    } else if (state && state.exists && !canVerifyExisting) {
        note.classList.add('is-warning');
        note.textContent = 'В выбранном каталоге обнаружен ServeHub-2, но его установленную версию не удалось полностью подтвердить. ' +
            (state.message || '') +
            ' Если продолжить стандартную установку, текущий каталог будет сохранён в резервной копии перед заменой.';
    } else if (canVerifyExisting) {
        note.textContent = 'Обнаружен существующий ServeHub-2: ' + state.metadata.display_name +
            ' (commit ' + state.metadata.commit_sha + '). Установщик автоматически использует этот репозиторий без повторной загрузки и замены кода. Для установки другой версии выберите пустую директорию.';
    } else {
        note.style.display = 'none';
        note.textContent = '';
    }

    const pathStatus = state && state.directory
        ? ('Каталог проверен: ' + state.directory + '.')
        : '';
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
        projectVersionCatalog = result;
        projectInstallState = result.installed;
        populateProjectVersionChoices(result, preserveChoice);
        renderInstalledProjectState(result.installed, !preserveChoice);
        projectVersionsLoaded = true;
        window.updateProjectActionUI();
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

    if (projectInstallState && projectInstallState.partial) {
        summary.textContent = 'В выбранном каталоге обнаружена неполная установка; автоматическая установка заблокирована.';
        warning.textContent = 'Выберите другой каталог или устраните повреждение вручную. Установщик не будет автоматически заменять непроверенные данные.';
        setProjectVersionContinueEnabled(false);
        return;
    }

    if (selectedProjectAction() === 'use') {
        summary.textContent = 'Обнаружен установленный ServeHub-2: ' +
            (projectInstallState?.metadata?.display_name || 'версия не указана') +
            '. Будет использован текущий проверенный репозиторий без загрузки новой версии.';
        warning.textContent = 'Выбор тега или dev не изменит существующую установку, пока выбран этот каталог.';
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
        warning.textContent = 'Установщик загрузит и проверит код до установки. ' +
            (projectInstallState && projectInstallState.exists
                ? 'Обнаруженный каталог ServeHub-2 будет сохранён в резервной копии, затем заменён проверенным кодом; данные контейнеров и БД этим действием не откатываются.'
                : 'ServeHub-2 в каталоге не обнаружен, поэтому будет создана новая установка. Файлы в непустом каталоге, не относящиеся к ServeHub-2, не будут перезаписаны.');
    } else {
        summary.textContent = 'Выберите поддерживаемый тег или ветку dev.';
        setProjectVersionContinueEnabled(false);
        return;
    }

    if (option.value === 'dev') {
        warning.textContent += ' ' + (projectInstallState && projectInstallState.exists
            ? 'После проверки новый код заменит текущий каталог, который сохранится в резервной копии.'
            : 'ServeHub-2 в каталоге не обнаружен, поэтому будет подготовлена новая установка. Файлы в непустом каталоге, не относящиеся к ServeHub-2, не будут перезаписаны.');
    }
    setProjectVersionContinueEnabled(true);
};

window.continueFromVersionSelection = async function() {
    if (!projectInstallDirectoryVerified) {
        updateProjectDirectoryStatus('Сначала проверьте каталог установки.', true);
        return;
    }
    if (projectInstallState && projectInstallState.partial) {
        const summary = document.getElementById('project-version-summary');
        if (summary) summary.textContent = 'Автоматическая установка заблокирована: в выбранном каталоге найдена неполная установка. Выберите другой каталог или устраните проблему вручную.';
        return;
    }
    if (selectedProjectAction() === 'use') {
        const state = projectInstallState;
        if (!(state && state.exists && state.complete && state.knownVersion && state.integrityVerified)) {
            updateProjectDirectoryStatus('Не удалось подтвердить существующий репозиторий. Проверьте каталог ещё раз.', true);
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

updateStepProgress(0);
window.initializeProjectInstallDirectory().then(() => window.initializeProjectVersions());