import './style.css';
import { CheckDocker, InstallProject, CloseApp } from '../wailsjs/go/main/App.js';

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
    statusText.style.color = '#e0e0e0';
    btnNext.disabled = true;
    btnRetry.style.display = 'none';

    await new Promise(resolve => setTimeout(resolve, 700));

    try {
        const result = await CheckDocker();

        spinner.style.display = 'none';

        if (result.success) {
            statusText.innerText = `✓ ${result.message}`;
            statusText.style.color = '#00ffcc';
            btnNext.disabled = false;
        } else {
            statusText.innerText = `✗ Ошибка: ${result.message}.`;
            statusText.style.color = '#ff5555';
            btnRetry.style.display = 'block';
        }
    } catch (err) {
        spinner.style.display = 'none';
        statusText.innerText = 'Критическая ошибка при вызове проверки бэкенда:\n' + err;
        statusText.style.color = '#ff5555';
        btnRetry.style.display = 'block';
    }
}

window.runInstallation = async function() {
    const installText = document.getElementById('install-text');
    const spinner = document.getElementById('install-loading');
    const btnFinish = document.getElementById('btn-finish');

    spinner.style.display = 'block';
    installText.innerText = 'Загрузка архива проекта ServeHub-2 с GitHub в память...';
    installText.style.color = '#e0e0e0';
    btnFinish.disabled = true;

    try {
        const result = await InstallProject();
        
        spinner.style.display = 'none';

        if (result.success) {
            installText.innerText = `✓ ${result.message}`;
            installText.style.color = '#00ffcc';
            btnFinish.disabled = false;
            btnFinish.innerText = 'Готово';
        } else {
            installText.innerText = `✗ Ошибка установки: ${result.message}`;
            installText.style.color = '#ff5555';
        }
    } catch (err) {
        spinner.style.display = 'none';
        installText.innerText = 'Критическая ошибка в процессе установки:\n' + err;
        installText.style.color = '#ff5555';
    }
}

window.closeInstaller = function() {
    CloseApp();
}