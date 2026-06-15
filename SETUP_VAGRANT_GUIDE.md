# Гайд по настройке тестирования Ansible на виртуальных машинах Vagrant
## ⚠️ Важно
- HashiCorp в данный момент не предоставляет доступ к своим сервисам из России, поэтому для скачивания образов и репозиториев обязательно понадобится активный VPN.

## Установка
### Linux
1. Установите необходимые компоненты:
    - **Docker** + **Docker Compose**
    - **Git**
    - **SSH**
    - **VirtualBox**
    - **Vagrant**
    - **Ansible**
2. Клонируйте репозиторий и перейдите в папку проекта:
   ```bash
   git clone [https://github.com/canntstand/ServeHub-2](https://github.com/canntstand/ServeHub-2) && cd ServeHub-2

```

3. Создайте файл конфигов `ansible/vars/secrets.yml` на основе примера `ansible/vars/secrets.yml.example` и заполните его своими данными.

### Windows (через WSL2)

1. Установите на Windows-хост: **Docker Desktop**, **Git**, **VirtualBox** и **Vagrant**.
2. Установите подсистему **WSL** (рекомендуется дистрибутив Ubuntu) и терминал с поддержкой Bash. Внутри самого дистрибутива WSL также необходимо установить пакет `vagrant`.
3. **Настройка сети (Проброс VPN в WSL):** Чтобы трафик из WSL безболезненно проходил через Windows-версию VPN, включите зеркальный режим сети. В терминале WSL выполните команду:
```bash
powershell.exe -Command "notepad \$HOME\.wslconfig"

```


В открывшемся Блокноте вставьте следующие строки, сохраните и закройте файл:
```ini
[wsl2]
networkingMode=mirrored
dnsTunneling=true
autoProxy=true

```


После этого перезапустите WSL в командной строке Windows (`wsl --shutdown`).
4. **Включение интеграции Vagrant с Windows:** Откройте терминал WSL и добавьте переменную окружения в ваш профиль:
```bash
echo 'export VAGRANT_WSL_ENABLE_WINDOWS_ACCESS="1"' >> ~/.bashrc
source ~/.bashrc

```


5. **Клонирование проекта:** ⚠️ *Важно!* Склонируйте репозиторий строго на физический диск Windows (в зону видимости VirtualBox), перейдя по пути `/mnt/c/`:
```bash
cd /mnt/c/
mkdir -p Projects && cd Projects
git clone [https://github.com/canntstand/ServeHub-2](https://github.com/canntstand/ServeHub-2) && cd ServeHub-2

```


6. Создайте файл конфигов `ansible/vars/secrets.yml` на основе примера `ansible/vars/secrets.yml.example`.

## Развертывание тестовой среды

Управление виртуальной лабораторией полностью автоматизировано через интерактивный скрипт. Запустите его в терминале:

```bash
chmod +x manage_deploy.sh
./manage_deploy.sh

```

Используйте следующие пункты меню:

* **Пункт 7: «Работа с Vagrant: Поднятие сред»** — Vagrant автоматически скачает из облака и запустит 3 независимые виртуальные машины: Arch Linux, Debian Bookworm и Ubuntu Jammy, а также подготовит для Ansible файлы SSH-доступа.
* **Пункт 8: «Деплой на Vagrant-узлы»** — запуск контейнера с Ansible для развертывания и тестирования всей вашей IT-инфраструктуры на созданных виртуалках.
* **Пункт 9: «Удаление Vagrant-сред»** — чистое удаление тестовых виртуальных машин из VirtualBox для освобождения ресурсов вашего ПК.