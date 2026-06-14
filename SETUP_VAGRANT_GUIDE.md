# Гайд по настройке тестирования Ansible на виртуальных машинах Vagrant.
1. Установить:
    - **Docker** + **Docker Compose** 
    - **Git**
    - **SSH** 
    - **VirtualBox**
    - **Vagrant**
    - **Ansible**
    - **WSL + Дистрибутив** (если на Windows)
    - **Терминал с поддержкой Bash** (если на Windows)
2. Клонировать репозиторий: `git clone https://github.com/canntstand/ServeHub-2 && cd ServeHub-2`
3. Создать файл `ansible/vars/secrets.yml` на основе примера `ansible/vars/secrets.yml.example`.
4. Запуск через `manage_deploy.sh`:
    - **Шаг 8:** `vagrant up` — создает и запускает 3 виртуальные машины (arch, debian, ubuntu).
    - **Шаг 9:** Запуск настройки и деплоя на них.
    - **Шаг 10:** Удаление машин