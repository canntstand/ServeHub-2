# Гайд по настройке тестирования Ansible на виртуальных машинах Vagrant.
## ⚠️ Важно
- HashiCorp в данный момент не работает в России, поэтому для запуска тестирования понадобится VPN

## Установка
1. Установить:
    - Если на Linux:
        - **Docker** + **Docker Compose** 
        - **Git**
        - **SSH** 
        - **VirtualBox**
        - **Vagrant**
        - **Ansible**
    - Если на Windows: (дополнительно к тому что установлено на Linux)
        - **WSL + Дистрибутив**
        - **Терминал с поддержкой Bash**
        - **Vagrant в WSL**
2. Чтобы трафик из WSL также проходил через VPN: (если на Windows)
    1. В терминале дистрибутива WSL: `powershell.exe -Command "notepad \$HOME\.wslconfig"`
    2. В блокноте написать:
        ```
        [wsl2]
        networkingMode=mirrored
        dnsTunneling=true
        autoProxy=true
        ```
3. Клонировать репозиторий: `git clone https://github.com/canntstand/ServeHub-2 && cd ServeHub-2`
4. Создать файл `ansible/vars/secrets.yml` на основе примера `ansible/vars/secrets.yml.example`.
5. Запуск через `manage_deploy.sh`:
    - **Шаг 7:** `vagrant up` — создает и запускает 3 виртуальные машины (arch, debian, ubuntu).
    - **Шаг 8:** Запуск настройки и деплоя на них.
    - **Шаг 9:** Удаление машин