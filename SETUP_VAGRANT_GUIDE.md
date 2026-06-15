# Гайд по настройке тестирования Ansible на виртуальных машинах Vagrant.
## ⚠️ Важно
- HashiCorp в данный момент не работает в России, поэтому для запуска тестирования понадобится VPN

## Установка
### Linux
1. Установить:
    - **Docker** + **Docker Compose** 
    - **Git**
    - **SSH** 
    - **VirtualBox**
    - **Vagrant**
    - **Ansible**
2. Клонировать репозиторий: `git clone https://github.com/canntstand/ServeHub-2 && cd ServeHub-2`
3. Создать файл `ansible/vars/secrets.yml` на основе примера `ansible/vars/secrets.yml.example`.
4. Запустить скрипт развертывания: `chmod +x manage_deploy.sh && ./manage_deploy.sh` (подробнее см. в разделе «Пользование сервисами»)


### Windows
1. Установить:
    - **Docker** + **Docker Compose** 
    - **Git**
    - **SSH** 
    - **VirtualBox**
    - **Vagrant**
    - **Ansible**
    - **WSL + Дистрибутив**
    - **Терминал с поддержкой Bash**
    - **Vagrant в WSL**
2. Зайти в терминал дистрибутива в WSL (все остальные пункты будут выполняться строго в нем)
3. Добавить переменную окружения в профиль: `echo 'export VAGRANT_WSL_ENABLE_WINDOWS_ACCESS="1"' >> ~/.bashrc && source ~/.bashrc`
4. Клонировать репозиторий (где угодно в /mnt/c/): `git clone https://github.com/canntstand/ServeHub-2 && cd ServeHub-2`
5. На основной системе в корне проекта выполнить `vagrant up`
6. Создать файл `ansible/vars/secrets.yml` на основе примера `ansible/vars/secrets.yml.example`.
7. Запустить скрипт развертывания: `chmod +x manage_deploy.sh && ./manage_deploy.sh` (подробнее см. в разделе «Пользование сервисами»)

### Пользование сервисами
- Использовать следующие пункты меню:
    - Пункт 7: «Работа с Vagrant: Поднятие сред» — Vagrant автоматически скачает из облака и запустит 3 независимые виртуальные машины: Arch Linux, Debian Bookworm и Ubuntu Jammy, а также подготовит для Ansible файлы SSH-доступа.
    - Пункт 8: «Деплой на Vagrant-узлы» — запуск контейнера с Ansible для развертывания и тестирования всей вашей IT-инфраструктуры на созданных виртуалках.
    - Пункт 9: «Удаление Vagrant-сред» — чистое удаление тестовых виртуальных машин из VirtualBox для освобождения ресурсов вашего ПК.