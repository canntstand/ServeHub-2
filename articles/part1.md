
# Своя self-hosted двухсерверная VPN-архитектура. Подробный путь разработки и ошибки, с которыми вы можете столкнуться

Всё началось с того, что меня перестал устраивать мой подход к развертыванию личных сервисов. Я решил переписать всё с нуля. К тому же это был отличный повод получить новые навыки и попробовать инструменты, до которых давно не доходили руки.

Арендовать мощные VPS под ресурсоемкие задачи в текущих реалиях выходит неоправданно дорого. При этом дома у меня уже был выделенный неттоп (мини-ПК) с 16 ГБ оперативной памяти на базе энергоэффективного процессора Intel N95.

Пройдя путь от простых Bash-скриптов и сторонних туннелей до полностью автоматизированной инфраструктуры, я создал проект ServeHub-2. В этой статье я подробно разберу, как эволюционировала сеть проекта, почему декларативный подход победил императивный и с какими неочевидными багами пришлось столкнуться в процессе автоматизации.

## История настройки сетевой архитектуры

### Использование Tuna

В самом начале проекта я еще не думал о VPN как о способе пробития NAT и основе, на которой будет строиться вся система. Первое, к чему я пришел — сервис Tuna. Если кратко, это аналог Cloudflare Tunnel за условные 300 рублей. У него очень приятный веб-интерфейс и невероятно простая настройка. Однако для полноценной независимой архитектуры он не подошел по двум причинам:

*   Сервера Tuna находятся вне контроля пользователя.
*   На удаленном сервере нельзя развернуть свои сопутствующие сервисы.

В целом сервис действительно удобный, но для моих задач он оказался слишком ограничивающим.

Вот пример конфига для tuna (все максимально просто, создаем контейнеры для ssh туннеля, чтобы был доступ ssh, а также основной http туннель с привязкой к nginx или любому другому прокси если такой есть):

```yaml
tuna_matrix:
    container_name: tuna_matrix
    image: yuccastream/tuna:latest
    restart: always
    environment:
      - TUNA_TOKEN=${TUNA_TOKEN}
      - SYNAPSE_SERVER_NAME=${SYNAPSE_SERVER_NAME}
    command: http nginx:80 --domain=${SYNAPSE_SERVER_NAME} --https-redirect
    depends_on:
      - nginx

  tuna_ssh:
    container_name: tuna_ssh
    image: yuccastream/tuna:latest
    restart: always
    network_mode: "host"
    environment:
      - TUNA_TOKEN=${TUNA_TOKEN}
    command: tcp 22 --port=${TUNA_SSH_PORT}
```

### В поисках гибкости: тесты Pangolin и переход к FRP

После Tuna я решил двигаться в сторону собственного контроля и попробовал Pangolin. Однако решение быстро отвалилось: для дешёвого сервера оно оказалось слишком «тяжёлым» и избыточным. Главные минусы — ощутимый оверхед по ресурсам и лишний слой принудительной веб-аутентификации перед самими сервисами. Это ломало нормальное взаимодействие с родными мобильными клиентами (вроде Element для Matrix или Bitwarden для паролей), где повторная авторизация в браузере просто не нужна.

На смену пришел FRP (Fast Reverse Proxy). Он выполнял ту же функцию проброса, но хостил я его уже на собственном арендованном VPS в режиме Layer 4 (TCP). Это дало абсолютную гибкость: внешний VPS не заглядывал внутрь пакетов и ничего не расшифровывал, а просто пересылал сырой поток домой. Кроме того, это отлично оптимизировало расходы: вместо 300 рублей за Tuna и ещё 300 рублей за отдельный VPN, я стал платить всего 500 рублей за один стойкий VPS, который мог настраивать как хочу. (к тому же можно было обойтись даже дешевле, так как VPS с 4 гб оперативки загружен всего лишь на 40%, процессор всего на 20%-40%)

FRP состоит из 2 конфигурационных файлов (один на удаленном сервере frp server, другой на локальном frp client), все также довольно просто, однако его можно использовать на разных слоях. У меня он брал трафик за стандартный TCP и передавал его на локальный сервер.

Также доп. фишка в том что основная конфигурация происходит именно в frpc, который, в свою очередь, передает часть настроек на frp на удаленном сервере.

**frpc.toml:**
```toml
serverAddr = "{{ .Envs.FRP_SERVER_IP }}"
serverPort = 7000
auth.method = "token"
auth.token = "{{ .Envs.FRP_TOKEN }}"

[[proxies]]
name = "http_traffic"
type = "tcp"
localIP = "nginx_proxy"
localPort = 80
remotePort = 8080
transport.proxyProtocolVersion = "v2"

[[proxies]]
name = "https_traffic"
type = "tcp"
localIP = "nginx_proxy"
localPort = 443
remotePort = 8443
transport.proxyProtocolVersion = "v2"

[[proxies]]
name = "secure_ssh"
type = "tcp"
localIP = "host.docker.internal"
localPort = 22
remotePort = 2222
```

**frps.toml:**
```toml
bindPort = 7000
auth.method = "token"
auth.token = "{{ .Envs.FRP_TOKEN }}"
maxPortsPerClient = 10
```

### Полноценный переезд на WireGuard (AmneziaWG) и 8 часов отладки

Со временем архитектура эволюционировала в сторону полноценной VPN-сети на базе WireGuard, а точнее — его модификации AmneziaWG. Причин для этого шага было несколько:

*   **Безопасность:** FRP всё же открывал внутренние ресурсы в публичный интернет, оставляя их доступными для сканеров портов.
*   **Удобство маршрутизации:** Все участники сети стали равноправными узлами в одной виртуальной локальной подсети. Больше не нужно было настраивать постоянные односторонние пробросы.
*   **Дополнительный бонус:** Поскольку удаленный VPS был куплен в Нидерландах, через этот же VPN я автоматически получил безопасный доступ ко всем зарубежным ресурсам.

Для реализации схемы на удаленном VPS был развернут контейнер `wg-easy` с поддержкой AmneziaWG, а на домашнем мини-ПК — клиентский контейнер Amnezia (сборка из Dockerfile с использованием amnezia-tools и модулем ядра хоста).

И именно здесь я поймал самый изнурительный баг проекта. После развертывания трафик упорно шёл только в одну сторону. Часов 8 ушло на диагностику iptables, маршрутов и чтение зарубежных форумов (что бесполезно, учитывая специфику наших блокировок). Оказалось, провайдер просто дропал обратный трафик стандартного WireGuard, так как пакеты шли без маскировки. Причина крылась в `docker-compose.yml`: у меня было прописано `image: ghcr.io/wg-easy/wg-easy:latest`. Как выяснилось, тег latest на Docker Hub намертво прилип к старой 14-й версии, а поддержка параметров AmneziaWG появилась только в ветке 15.x. Изменение тега на конкретную версию (15.3) решило проблему за секунду.

Благодаря переходу конфигурация получилась максимально простой, основные отличия от стандартной документации выделил в коде: (в основном то, на что пришлось долго рыть информацию)

```yaml
 wg-easy:
    image: ghcr.io/wg-easy/wg-easy:15.3 # самая последняя версия
    container_name: wg-easy
    environment:
      - LANG=en
      - WG_HOST=${VPS_PUBLIC_IP}
      - PORT=51821
      - WG_PORT=51820
      - WEBROOT_BIND_ADDRESS=127.0.0.1
      - WG_DEFAULT_ADDRESS=10.8.0.x
      - INIT_DNS=10.8.0.1 # это важно для adguard home (об этом позже), так как он именно на этом ip
      - WG_PERSISTENT_KEEPALIVE=25
      - WG_ALLOWED_IPS=0.0.0.0/0
      - EXPERIMENTAL_AWG=true # включаем поддержку amnezia, нужно будет установить модуль ядра на хост
      - OVERRIDE_AUTO_AWG=awg # чтобы не использовался стандартный модуль wg
      - INSECURE=true # оставили INSECURE, так как есть прокси
      - WG_DEVICE=eth0
      - INIT_ENABLED=true
      - INIT_USERNAME=${ADMIN_USER}
      - INIT_PASSWORD=${ADMIN_PASSWORD}
      - INIT_HOST=${VPS_PUBLIC_IP}
      - INIT_PORT=51820
    volumes:
      - ./apps-data/wg-easy:/etc/wireguard
      - /lib/modules:/lib/modules:ro
    network_mode: "host"
    restart: unless-stopped
    cap_add:
      - NET_ADMIN
      - SYS_MODULE
    healthcheck:
      test: ["CMD", "awg", "show", "wg0"]
      interval: 30s
      timeout: 10s
      retries: 3
      start_period: 10s
```

### Настройка Nginx

Чтобы сервисы были доступны исключительно внутри подсети VPN, я задействовал Nginx. Доступ к приложениям был жестко ограничен на уровне конфигурации — веб-сервер принимает запросы только из диапазона IP-адресов `10.8.0.0/24`. Любые попытки постучаться на сервер из внешнего интернета без активного VPN-туннеля автоматически сбрасываются Nginx.

Логика распределения завязана на proxy-протоколе: Nginx на удаленном VPS выступает основным входным узлом, принимает зашифрованный трафик, заворачивает его в заголовки с реальным IP-адресом клиента и через туннель перекидывает на локальный Nginx домашнего сервера. Локальный веб-сервер уже сам расшифровывает SSL и распределяет трафик по конечным Docker-контейнерам, сохраняя реальные IP в логах безопасности.

Вставлю один кусок кода из nginx на удаленной машине для примера, так все остальное примерно похоже (конфиги использовались в виде .template):

```nginx
map $http_upgrade $connection_upgrade {
    default upgrade;
    ''      close;
} # здесь умная обработка websockets, чтобы соединение апгрейдилось только когда нужно

server {
    listen 443 ssl; # указываем ssl
    server_name adguard.${SERVER_NAME}; # использую шаблоны

    ssl_certificate /etc/letsencrypt/live/${SERVER_NAME}/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/${SERVER_NAME}/privkey.pem;

    client_max_body_size 2G; # решил указать для всех сервисов, так как некоторые требуют перекачки больших файлов
    add_header Strict-Transport-Security "max-age=15552000; includeSubDomains; preload" always;
    set_real_ip_from 172.16.0.0/12; # ограничиваем доверенные ip
    set_real_ip_from 10.0.0.0/8;
    set_real_ip_from 127.0.0.1;
    real_ip_recursive on; # это нужно там где nginx перебрасывает трафик на локальный сервер, чтобы другой сервер видел ip

    allow 127.0.0.1;  # здесь разрешаются только пользователи из определенных диапазонов адресов
    allow 172.16.0.0/12;
    allow 10.8.0.0/24;
    deny all;

    location / {
        proxy_pass http://127.0.0.1:8082; # пересылает трафик в контейнер
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection $connection_upgrade;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto https;
    }
}
```

### SSL-сертификаты

Для получения валидных SSL-сертификатов я настроил работу через автоматический Certbot по challenge-валидации `DNS-01` c API Webnames. Сам домен привязан к внутреннему IP-адресу 10.8.0.1. Проверка через DNS позволила выпустить единый wildcard-сертификат на весь домен и его поддомены без необходимости держать открытым 80-й порт веб-сервера наружу.

Уточню, что certbot запускается автоматически во время выполнения Ansible плейбука, поэтому самому кроме указания переменных ничего делать не нужно.

```yaml
# =========================================================================
# 3. НАСТРОЙКА ПЛАГИНА WEBNAMES DNS ДЛЯ CERTBOT
# =========================================================================
- name: "[Certbot] Клонирование плагина certbot-dns-webnames"
  ansible.builtin.git:
    repo: "https://github.com/regtime-ltd/certbot-dns-webnames.git"
    dest: "/home/{{ deploy_user }}/ServeHub-2/certbot/certbot-dns-webnames"
    version: master
    update: no

- name: "[Certbot] Скачивание конфигурации Certbot через API Webnames"
  ansible.builtin.get_url:
    url: "https://www.webnames.ru/scripts/json_domain_zone_manager.pl?action=get_config_certbot&domain={{ server_name }}&apikey={{ webnames_apikey }}"
    dest: "/home/{{ deploy_user }}/ServeHub-2/certbot/certbot-dns-webnames/config.sh"
    mode: "0755"

- name: "[Certbot] Валидация: Проверка, что полученный файл конфигурации не пустой"
  ansible.builtin.stat:
    path: "/home/{{ deploy_user }}/ServeHub-2/certbot/certbot-dns-webnames/config.sh"
  register: webnames_config_stat
  failed_when: webnames_config_stat.stat.size == 0

- name: "[Certbot] Установка прав на исполнение для всех скриптов плагина"
  ansible.builtin.shell:
    cmd: "chmod +x *.sh"
    chdir: "/home/{{ deploy_user }}/ServeHub-2/certbot/certbot-dns-webnames"
  changed_when: true
```

В итоге получилась схема, при которой все сервисы доступны по красивым доменным именам с HTTPS, но абсолютно невидимы для внешнего интернета.

Вот настройка certbot:

```yaml
  certbot:
    build:
      context: .
      dockerfile: Dockerfile.certbot
    container_name: certbot
    logging: *default-logging
    volumes:
      - ./certbot/certs:/etc/letsencrypt
      - ./certbot/certs-data:/var/lib/letsencrypt
      - ./certbot/certbot-dns-webnames:/opt/certbot-dns-webnames
    environment:
      - SERVER_NAME=${SERVER_NAME}
    command: >
      certonly
      --manual
      --manual-auth-hook /opt/certbot-dns-webnames/authenticator.sh # использую проверку по DNS, чтобы не нужно было ничего открывать в интернет
      --manual-cleanup-hook /opt/certbot-dns-webnames/cleanup.sh
      --preferred-challenges dns-01
      --agree-tos
      --no-eff-email
      --non-interactive
      --force-renewal
      -d "${SERVER_NAME}"
      -d "*.${SERVER_NAME}"
      --email "${EMAIL}"
    deploy:
      resources:
        limits:
          memory: 1G
```

## История софта

Параллельно с сетевой структурой развивался и сам набор приложений. Изначально я хотел собрать в одном месте утилиты, которыми пользуюсь каждый день, но в процессе селфхостинга быстро понимаешь: нельзя просто накидать контейнеров и надеяться, что мини-ПК справится, а конфигурационные файлы не превратятся в кашу.

### Первый стек и оптимизация

Первыми на домашнем сервере прижились медиа-сервисы: Navidrome для стриминга музыки и Audiobookshelf для аудиокниг и подкастов. Они легковесные, имеют отличные мобильные клиенты с синхронизацией прогресса и полностью закрывают мои потребности. Позже к ним добавился Nextcloud как единое независимое облако для файлов, контактов и семейных документов.

Затем встал вопрос безопасного хранения паролей. Сначала я смотрел в сторону оригинального Bitwarden, но в итоге я выбрал Vaultwarden — альтернативный сервер на Rust, полностью совместимый с API Bitwarden. Он потребляет считанные мегабайты оперативной памяти и работает идеально. Дополнительно для удобства управления всей этой распределенной Docker-инфраструктурой в локальный стек был добавлен Portainer. (+ Portainer Agent на удаленный сервер)

Из интересных моментов, где мне пришлось немного больше возиться, чем с остальными сервисами это nextcloud настройка:

```yaml
 nextcloud:
    image: nextcloud:34.0.0-apache
    container_name: nextcloud
    logging: *default-logging
    restart: unless-stopped
    expose:
      - 80
    depends_on:
      nextcloud-db:
        condition: service_healthy
    environment:
      - POSTGRES_HOST=nextcloud-db
      - POSTGRES_DB=${POSTGRES_DB_NEXTCLOUD}
      - POSTGRES_USER=${POSTGRES_USER}
      - POSTGRES_PASSWORD=${POSTGRES_PASSWORD}
      - NEXTCLOUD_ADMIN_USER=${ADMIN_USER}
      - NEXTCLOUD_ADMIN_PASSWORD=${ADMIN_PASSWORD}
      - OVERWRITEWEBROOT=/nextcloud # важно для работы nextcloud не в корне сервера
      - OVERWRITEPROTOCOL=https
      - HTACCESS_OVERRIDE_CURRENT=1
      - NEXTCLOUD_TRUSTED_DOMAINS=${SERVER_NAME}
      - TRUSTED_PROXIES=172.16.0.0/12 10.8.0.0/24
      - NC_maintenance_window_start=2 # задаем окно обслуживания приложения, когда запускать фоновые задачи
      - NC_default_phone_region=RU
      - NC_filelocking.enabled=true # Блокировка файлов во время их изменения 
      - 'NC_memcache.local=\OC\Memcache\APCu' # Включает локальное кэширование данных с помощью APCu, возможно потом добавлю Redis
    volumes:
      - ./apps-data/nextcloud/html:/var/www/html
      - /home/${LOCAL_USER}/PersonalData/NextcloudData:/var/www/html/data
    deploy:
      resources:
        limits:
          memory: 3G
    healthcheck:
      test:
        [
          "CMD-SHELL",
          "test -f /var/www/html/config/config.php && php occ status",
        ]
      interval: 1m30s
      timeout: 30s
      retries: 5
      start_period: 30s
```

### Ошибки проектирования: почему я удалил Matrix (Synapse)

Не все решения прошли проверку временем. На этапе использования Tuna и FRP я развернул сервер Matrix (Synapse) для защищенного обмена сообщениями. Мне казалось это крутой идеей, но когда я окончательно перешел на AmneziaWG, целесообразность мессенджера внутри закрытого туннеля сошла на нет.

Synapse требовал слишком много ресурсов, впустую расходовал оперативку домашнего ПК и усложнял конфиг Nginx. При этом реальной пользы для семьи он не приносил. Для критических алертов инфраструктуры и повседневного общения проще и эффективнее оказалось использовать Telegram. (плюс алертинг настроен именно через него) В итоге я полностью выпилил Synapse из стека, освободив ресурсы.

Вместо него я добавил в связку к `wg-easy` локальный AdGuard Home. Теперь он работает прямо внутри VPN-сети: очищает весь трафик от рекламы и трекеров на лету, кэширует DNS-запросы и не дает истории веб-серфинга улетать внешним провайдерам.

Основной проблемой с которой я столкнулся при использовании AdGuard Home, так это то что я так и не понял как заставить использовать AmneziaVPN клиент AdGuard как основной DNS, при этом AmneziaWG работает прекрасно. Как я понимаю дело в том что AmneziaWG работает намного проще на уровне ip и у него нету никаких доп фильтров, настроек и подобного, поэтому он просто берет данные из конфига.

### От костылей на Bash к декларативному Ansible

Весь стек на обоих серверах разворачивался через bash скрипты, что было уж очень плохо с точки зрения идемпотентности. Во первых из-за bash мне постоянно приходилось очищать сервера, так как нормальных проверок у меня не было и писать я их не хотел, а также было много костылей с импортом переменных, записями в файлы и подобным.

Так я пришел к декларативному подходу и Ansible. Теперь вся конфигурация описывается в виде плейбуков и ролей, отражающих конечное желаемое состояние серверов. Конфиденциальные данные перенесены в файл `secrets.yml`, а хрупкие конструкции автоматизированы через шаблоны Jinja2. Проект стал идемпотентным: если шаги уже выполнены, Ansible их просто пропускает.

В итоге получилось несколько yaml файлов для стандартной настройки системы (`bootstrap_os.yml`) и для настройки каждого из хостов (`setup_local.yml` и `setup_remote.yml`). В итоге теперь все что нужно чтобы полностью с нуля развернуть проект - скачать Ansible, несколько других зависимостей на свой рабочий пк и запустить один `manage_deploy.sh`, в котором можно будет выбрать сценарий как будет вести себя Ansible и спокойно дождаться разворачивания сервисов.

Также благодаря Ansible я удобно реализовал переносимость проекта. Так как я решил не использовать тома docker, и вместо этого храню все в папках, чтобы перенести старые данные проекта нужно просто скопировать папку `apps-data` и положить ее в нужное место и все само заработает после повторного развертывания проекта. В Ansible выделяется отдельная пауза для этого.

Вот пример основного плейбука `deploy.yml`:

```yaml
- name: "[Bootstrap] Проверка: доступен ли root?"
  hosts: vps, local
  gather_facts: no
  vars_files:
    - vars/secrets.yml
  tags: bootstrap

  vars:
    ansible_password: "{{ user_pass }}"
    ansible_ssh_pass: "{{ user_pass }}"

  pre_tasks:
    - name: "[SSH] Проверка: доступен ли root?"
      ansible.builtin.ping:
      ignore_errors: yes
      ignore_unreachable: yes
      vars:
        ansible_user: root
      register: root_auth_check

    - name: "[SSH] Логика: настройка подключения под root"
      set_fact:
        ansible_user: root
        ansible_become: no
      when: root_auth_check.ping is defined

    - name: "[SSH] Логика: настройка sudo для обычного пользователя"
      set_fact:
        ansible_become: yes
        ansible_become_method: sudo
        ansible_become_password: "{{ user_pass }}"
      when: root_auth_check.ping is not defined

  tasks:
    - name: "[Python] Установка Python3 (необходим для работы Ansible)"
      raw: |
        if [ "$USER" != "root" ]; then SUDO="sudo"; else SUDO=""; fi

        if command -v apt-get >/dev/null; then
            $SUDO apt-get update && $SUDO env DEBIAN_FRONTEND=noninteractive NEEDRESTART_MODE=l apt-get install -y python3
        elif command -v pacman >/dev/null; then
            $SUDO pacman -Sy --noconfirm --overwrite "*" archlinux-keyring && $SUDO pacman -Syu --noconfirm --overwrite "*" python3
        else
            echo "Unsupported distribution" >&2
            exit 1
        fi
      changed_when: false

    - name: "[Facts] Сбор системных фактов после установки Python"
      setup:

    - name: "[Общее] Проверка поддерживаемой ОС (Debian/Ubuntu или Arch)"
      fail:
        msg: "Неподдерживаемая ОС. Требуется Debian/Ubuntu или Arch Linux"
      when:
        - ansible_facts['os_family'] != 'Debian'
        - ansible_facts['os_family'] != 'Archlinux'

    - name: "[Bootstrap] Запуск сценария базовой настройки ОС"
      include_tasks: tasks/bootstrap_os.yml

  handlers:
    - name: Restart SSH
      ansible.builtin.systemd:
        name: "{{ 'ssh' if ansible_facts['os_family'] == 'Debian' else 'sshd' }}"
        state: restarted

- name: "[Git] Синхронизация репозитория ServeHub-2"
  hosts: vps, local
  vars_files:
    - vars/secrets.yml
  become: yes
  tasks:
    - name: "[Git] Клонирование или обновление репозитория ServeHub-2"
      ansible.builtin.git:
        repo: "{{ project_repo_url }}"
        dest: "/home/{{ deploy_user }}/ServeHub-2"
        clone: yes
        update: yes
        force: yes
      become: true
      become_user: "{{ deploy_user }}"

    - name: "[Общее] Генерация файла .env из шаблона env.j2"
      ansible.builtin.template:
        src: env.j2
        dest: "/home/{{ deploy_user }}/ServeHub-2/.env"
        owner: "{{ deploy_user }}"
        group: "{{ deploy_user }}"
        mode: "0600"

- name: "Настройка удалённой инфраструктуры (VPS)"
  hosts: vps
  vars_files:
    - vars/secrets.yml
  become: yes
  tasks:
    - name: "[VPS] Запуск нативных задач настройки удалённого сервера"
      include_tasks: tasks/setup_remote.yml

- name: "Настройка локальной инфраструктуры (Local Server)"
  hosts: local
  vars_files:
    - vars/secrets.yml
  become: yes
  tasks:
    - name: "[Local] Запуск нативных задач настройки локального сервера"
      include_tasks: tasks/setup_local.yml
```

## Тестирование мультидистрибутивности с помощью Vagrant

Проект изначально затачивался под работу на трех дистрибутивах: Ubuntu, Debian и Arch Linux. Тестировать Ansible-плейбуки прямо на рабочей локальной машине (в моем случае — EndeavourOS) слишком рискованно, а создавать виртуальные машины руками — долго и неудобно.

Решением стал Vagrant, позволяющий за пару минут развернуть чистые ОС в VirtualBox из готовых образов. Но в процессе настройки мультивендорного стенда в режиме сетевого моста (`public_network`) всплыли две критические проблемы:

1.  **Конфликт DNS:** По умолчанию Vagrant создает NAT-интерфейс для управления нодой. При включении второго (публичного) интерфейса для локальной сети ломался дефолтный DNS-резолвер. Проблему пришлось решать принудительной очисткой и перезаписью файла `/etc/resolv.conf` через inline-скрипт автоматизации Vagrant.
2.  **Проблема с GRUB на Debian:** В используемом базовом образе `generic/debian12` конфигурация GRUB сохраняла жесткую привязку к конкретному имени диска из окружения сборщика. При повторном развертывании плейбуков на тестовом стенде это приводило к сбоям загрузчика. Чтобы автоматизировать очистку, пришлось внедрить скрипт, который на лету определяет имя системного диска через `lsblk` и автоматически передает правильные параметры в загрузчик через утилиту `debconf-set-selections`.

Вот код Vagrantfile: (в `node.vm.provision` происходит основное решение ошибок)

```ruby
Vagrant.configure("2") do |config|
  boxes = {
    "arch-node"    => { box: "generic/arch" },
    "debian-node"  => { box: "generic/debian12" },
    "ubuntu-node"  => { box: "generic/ubuntu2204" }
  }

  boxes.each do |name, cfg|
    config.vm.define name do |node|
      node.vm.box = cfg[:box]
      node.vm.network "public_network", use_dhcp_assigned_default_route: true

      node.vm.provision "shell", inline: <<-SHELL
        echo "=== [Vagrant Fix] Настройка стабильного DNS для моста ==="
        rm -f /etc/resolv.conf
        echo -e "nameserver 8.8.8.8\nnameserver 1.1.1.1" > /etc/resolv.conf

        if [ -f /etc/debian_version ]; then
          echo "=== [Vagrant Fix] Исправление привязки GRUB к диску ==="
          PRIMARY_DISK=$(lsblk -ndrio NAME,TYPE | awk '$2=="disk" {print "/dev/"$1; exit}')

          echo "grub-pc grub-pc/install_devices string $PRIMARY_DISK" | debconf-set-selections
        fi
      SHELL

      node.vm.provider "virtualbox" do |vb|
        vb.memory = "8192"
        vb.cpus = 4
        vb.name = "servehub-test-#{name}"
        vb.customize ["modifyvm", :id, "--graphicscontroller", "vmsvga"]
      end
    end
  end
end
```

## Надежная система бэкапов на базе Borgmatic

Для создания резервных копий я внедрил Borgmatic (удобную надстройку над дедуплицирующим инструментом Borg Backup). Весь процесс автоматизирован с помощью связки системных юнитов `borgmatic.service` и `borgmatic.timer`. В бэкап уходят две ключевые директории: `apps-data` (конфигурации приложений и баз данных) и `PersonalData` (медиатека: музыка, книги, подкасты, файлы Nextcloud).

Развертывание системы бэкапов полностью берет на себя Ansible. Мне достаточно указать UUID внешнего жесткого диска — скрипт сам проверит его наличие в системе, примонтирует в нужную директорию, создаст зашифрованный репозиторий и настроит политику ротации (хранение 7 ежедневных, 4 еженедельных и 6 ежемесячных копий). Также в Prometheus выведен мониторинг самого репозитория `.borg` для отслеживания его размера и статуса успешности архивации.

Главная проблема при бэкапе работающих Docker-контейнеров — риск скопировать базу данных в «битом» или неконсистентном состоянии, если в момент создания архива в нее шла активная запись. Чтобы решить эту проблему, я задействовал механизм хуков в конфигурации Borgmatic.

Перед началом резервного копирования автоматически срабатывает команда остановки контейнеров проекта (`docker compose down`), а после успешного завершения (или в случае возникновения непредвиденной ошибки) контейнеры автоматически поднимаются обратно в фоновом режиме. Для удобного просмотра архивов и быстрого восстановления файлов я развернул веб-интерфейс Borg UI.

**Конфигурация borgmatic:** (использую как Jinja2 шаблон, чтобы Ansible в плейбуках сам подставил переменные)

```yaml
source_directories: # не использую тома docker для легкой переносимости и бэкапов
  - /home/{{ local_user }}/ServeHub-2/apps-data/
  - /home/{{ local_user }}/PersonalData/Music
  - /home/{{ local_user }}/PersonalData/NextcloudData
  - /home/{{ local_user }}/PersonalData/Books
  - /home/{{ local_user }}/PersonalData/Podcasts
  - /home/{{ local_user }}/PersonalData/Audiobooks

repositories:
  - path: /mnt/backup_storage/server_backup.borg

keep_daily: 7
keep_weekly: 4
keep_monthly: 6

compression: lz4

encryption_passphrase: {{ borgmatic_encryption_passphrase }}

# Тут просто перед бэкапом все останавливается, потом стартует заново
commands:
  - before: action
    when:
      - create
    run:
      - echo "Starting backup pipeline..."
      - cd /home/{{ local_user }}/ServeHub-2 && docker compose -f docker-compose.local.yaml down

  - after: action
    when:
      - create
    states:
      - finish
    run:
      - cd /home/{{ local_user }}/ServeHub-2 && docker compose -f docker-compose.local.yaml up -d
      - echo "Backup completed successfully!"

  - after: error
    run:
      - cd /home/{{ local_user }}/ServeHub-2 && docker compose -f docker-compose.local.yaml up -d
```

## Наблюдаемость (Observability) уровня Enterprise

В последних релизах (v1.2.0 и v1.3.0) фокус проекта сместился на мониторинг и работу с логами.

### Эволюция алертинга и переход на Gatus

Изначально для мониторинга доступности я смотрел на Uptime Kuma, но отказался из-за отсутствия удобной декларативной настройки через конфиги. Затем я развернул связку Blackbox Exporter и Alertmanager с уведомлениями в Matrix. Но тут крылась логическая несостыковка: весь алертинг был завязан на локальном ПК, и в случае его аппаратного отказа я бы просто лишился уведомлений.

Тогда я решил вернуть Uptime Kuma, но развернуть его на удаленном VPS в качестве внешнего «сторожа» и автоматизировать его настройку через Python-скрипт с библиотекой `uptime-kuma-api`. Но и тут ждали «грабли» — библиотека не обновлялась три года и намертво ломалась на свежих версиях Kuma.

В итоге идеальным решением стал Gatus. Он изначально проектировался под управление через YAML-конфиги и поддерживает отправку алертов, если сервер не отвечает. Из-за специфики фронтенда Gatus (он не умеет работать из подкаталога типа `/gatus`), мне пришлось перенести его и wg-easy на полноценные субдомены `gatus.` и `wireguard.`.

Для Gatus получился простой конфиг:

```yaml
web:
  address: "127.0.0.1"
  port: 51822

alerting: # использую телеграм для алертов
  telegram:
    token: "${TELEGRAM_TOKEN}"
    id: "${TELEGRAM_CHAT_ID}"

endpoints:
  - name: "Is Local Server Alive?"
    url: "http://host.docker.internal:8080/health-check"
    interval: 60s
    conditions:
      - "[STATUS] == 200"
    alerts:
      - type: telegram
        failure-threshold: 3
        success-threshold: 2
        send-on-resolved: true
        description: "Локальный домашний сервер недоступен!"

  - name: "Is VPS Server Alive?"
    url: "http://host.docker.internal:8080/vps-health-check"
    interval: 60s
    conditions:
      - "[STATUS] == 200"
    alerts:
      - type: telegram
        failure-threshold: 3
        success-threshold: 2
        send-on-resolved: true
        description: "Веб-окружение VPS не отвечает!"

  - name: SSL Certificate
    url: "https://${SERVER_NAME}"
    interval: 1h
    conditions:
      - "[CERTIFICATE_EXPIRATION] > 720h"
    alerts:
      - type: telegram
        description: "SSL сертификат скоро истекает"
        failure-threshold: 3
        success-threshold: 2
        send-on-resolved: true
```

Для мониторинга аппаратных ресурсов удаленного и локального серверов была развернута связка `node-exporter` и `cAdvisor`. Все уведомления теперь приходят мгновенно в Telegram-бота. Чтобы избежать лавины одинаковых сообщений (например, при перезагрузке хоста), в Alertmanager настроена жесткая группировка и дедупликация событий. Также добавлен экспортер для AdGuard Home, выводящий статистику заблокированных запросов в Grafana.

### Централизованные логи: Loki + Grafana Alloy

В релизе v1.3.0 в стек была добавлена централизованная система сбора логов Loki. Вместо устаревшего Promtail в качестве агента сбора я применил Grafana Alloy.

Он эффективно собирает логи со всех запущенных Docker-контейнеров, парсит их и передает в Loki. Теперь вся история событий, ошибок веб-сервера Nginx или падений внутренних приложений доступна в едином интерфейсе Grafana с возможностью удобной фильтрации через LogQL, что значительно упрощает отладку.

**Конфиг Loki:**
```yaml
auth_enabled: false

server:
  http_listen_port: 3100
  grpc_listen_port: 9096

common:
  instance_addr: 0.0.0.0
  path_prefix: /loki
  storage:
    filesystem:
      chunks_directory: /loki/chunks
      rules_directory: /loki/rules
  replication_factor: 1
  ring:
    kvstore:
      store: inmemory

schema_config:
  configs:
    - from: 2024-01-01
      store: tsdb
      object_store: filesystem
      schema: v13
      index:
        prefix: index_
        period: 24h

limits_config:
  allow_structured_metadata: true
  reject_old_samples: true
  reject_old_samples_max_age: 168h
  retention_period: 30d

compactor:
  working_directory: /loki/compactor
  compaction_interval: 10m
  retention_enabled: true
  delete_request_store: filesystem
  retention_delete_delay: 2h
  retention_delete_worker_count: 150
```

**Конфиг Grafana Alloy:** (локальный конфиг)
```yaml
discovery.docker "local_containers" { 
  host = "unix:///var/run/docker.sock" # cканируем локальный docker-демон для поиска запущенных контейнеров
} 

discovery.relabel "docker_logs_relabel" {
  targets = discovery.docker.local_containers.targets

  rule {
    source_labels = ["__meta_docker_container_name"]
    regex         = "/(.*)"
    target_label  = "container_name"
  } # Убираем ведущий слэш "/" из имени контейнера (например, /nextcloud -> nextcloud). Это было очень важно, без этого ничего не работало

  rule {
    target_label = "server"
    replacement  = "local-minipc"
  } # некоторые сервисы на локальном и удаленном серверах имеют одинаковые имена, поэтому добавил пометки чтобы можно было понять с какого сервера логи
}

# собираем логи из Docker на основе подготовленного списка контейнеров
loki.source.docker "docker_logs" {
  host       = "unix:///var/run/docker.sock"
  targets    = discovery.relabel.docker_logs_relabel.output
  forward_to = [loki.write.loki_storage.receiver]
}

# отправляем логи в Loki
loki.write "loki_storage" {
  endpoint {
    url = "http://loki:3100/loki/api/v1/push" # для удаленного сервера тут просто ip 10.8.0.2
  }
}
```

## Интерфейс: переход на Homepage

Изначально для домашней страницы я написал кастомную минималистичную HTML-панель с Glassmorphism-дизайном. Выглядело это красиво, но добавлять новые сервисы вручную через постоянную правку исходного кода было крайне неудобно.

В итоге я заменил самописную страницу на полноценный комьюнити-проект Homepage. Это дало некоторую гибкость: вся панель настраивается через простые YAML-файлы и поддерживает встроенные виджеты интеграции. Пока что она простенькая, но возможно в будущем сделаю что-то более продвинутое.

## Заключение

Постарался подробно рассказать о структуре проекта и как я к этому пришел, очень много я не добавил, если пост зайдет, то я обязательно более подробно пройдусь по некоторым моментам: как я настраивал Matrix и на каком моменте я решил его убрать, как собирал локальный amnezia контейнер-клиент, что нового я добавил в релизе 1.4.0 и другое.

Также я планирую перейти с `manage_deploy.sh` на отдельный GUI клиент, который будет полностью автоматизировать процесс подготовки к deploy. Собираюсь использовать Go + Wails). Про это тоже возможно сделаю статью.