import './style.css';
import { CheckDocker, InstallProject, SaveSecrets, CloseApp, MinimizeApp, MaximizeApp, RunDeployment, CheckSecrets, SendEnter } from '../wailsjs/go/main/App.js';
import { EventsOn } from '../wailsjs/runtime/runtime.js';

let generatedYamlString = "";

window.goToStep = function(stepNumber) {
    document.querySelectorAll('.step').forEach(step => {
        step.classList.remove('active');
    });

    const nextStep = document.getElementById(`step-${stepNumber}`);
    if (nextStep) {
        nextStep.classList.add('active');
    }

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

    const btnStep6Back = document.querySelector('#step-6 .btn-secondary');

    spinner.style.display = 'block';
    installText.innerText = 'Загрузка архива проекта ServeHub-2 с GitHub в память...';
    installText.style.color = '#9a9aa0';
    btnFinish.disabled = true;

    try {
        const result = await InstallProject();
        
        if (result.success) {
            const secretsCheck = await CheckSecrets();
            
            spinner.style.display = 'none';

            if (secretsCheck.success) {
                installText.innerHTML = `✓ ${result.message}<br><br><span style="color: #a5b4fc; font-weight: bold;">ℹ Конфигурация найдена:</span> Файл <code style="background: #1e1e22; padding: 2px 6px; border-radius: 4px;">secrets.yml</code> уже существует в проекте. Шаги создания и настройки будут пропущены.`;
                installText.style.color = '#818cf8';
                
                btnFinish.disabled = false;
                btnFinish.innerText = 'Перейти к выбору развертки';
                btnFinish.setAttribute('onclick', 'goToStep(6)');

                if (btnStep6Back) {
                    btnStep6Back.setAttribute('onclick', 'goToStep(3)');
                }

            } else {
                installText.innerText = `✓ ${result.message}`;
                installText.style.color = '#818cf8';
                
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
            installText.style.color = '#e5484d';
        }
    } catch (err) {
        spinner.style.display = 'none';
        installText.innerText = 'Критическая ошибка в процессе установки:\n' + err;
        installText.style.color = '#e5484d';
    }
}

window.generateAndReviewYaml = function() {
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

    const formattedPrivateKey = privatesshKey
        .split('\n')
        .map(line => '    ' + line)
        .join('\n');

    generatedYamlString = `# ==============================================================================
#                 ServeHub-2: СГЕНЕРИРОВАННЫЙ КОНФИГУРАЦИОННЫЙ SECRETS.YML
# ==============================================================================

# ------------------------------------------------------------------------------
# 1. СЕТЕВАЯ ИНФРАСТРУКТУРА И ПОЛЬЗОВАТЕЛИ
# ------------------------------------------------------------------------------
vps_public_ip: "${vpsIp}"
vps_user: "${vpsUser}"
vps_root_password: "${vpsRootPass}"
local_private_ip: "${localIp}"
local_user: "${localUser}"
local_root_password: "${localRootPass}"

# ------------------------------------------------------------------------------
# 2. ГЛОБАЛЬНЫЕ НАСТРОЙКИ И АДМИНИСТРИРОВАНИЕ
# ------------------------------------------------------------------------------
server_name: "${serverName}"
admin_user: "${adminUser}"
admin_password: "${adminPass}"
email: "${email}"

# ------------------------------------------------------------------------------
# 3. НАСТРОЙКИ АЛЕРТИНГА (УВЕДОМЛЕНИЯ ЧЕРЕЗ TG БОТА ДЛЯ GATUS / СЕРВИСОВ)
# ------------------------------------------------------------------------------
telegram_token: "${tgToken}"
telegram_chat_id: "${tgChatId}"

# ------------------------------------------------------------------------------
# 4. ИНТЕГРАЦИЯ С DNS-ПРОВАЙДЕРОМ (АВТО-SSL)
# ------------------------------------------------------------------------------
webnames_apikey: "${webnamesKey}"

# ------------------------------------------------------------------------------
# 5. НАСТРОЙКИ СУБД POSTGRESQL
# ------------------------------------------------------------------------------
postgres_db_nextcloud: "${pgNc}"
postgres_db_vaultwarden: "${pgVw}"
postgres_user: "${pgUser}"
postgres_password: "${pgPass}"
nextcloud_redis_pass: "${redisPass}"

# ------------------------------------------------------------------------------
# 6. СЕКРЕТЫ И КЛЮЧИ БЕЗОПАСНОСТИ СЕРВИСОВ
# ------------------------------------------------------------------------------
secret_vaultwarden_password: "${vwAdminPass}"
ssh_public_key: "${publicsshKey}"
ssh_private_key: |
${formattedPrivateKey}
borgmatic_encryption_passphrase: "${borgPass}"
backup_disk_uuid: "${diskUuid}"
`;

    document.getElementById('yaml-preview').innerText = generatedYamlString;
    
    goToStep(5);
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