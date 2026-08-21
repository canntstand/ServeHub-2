# ЧАСТЬ 2. Своя self-hosted двухсерверная VPN-архитектура: Gitea, сборка Amnezia, проблемы 7.1.3-arch1-2 и многое другое

Эта статья является прямым продолжением, которое охватывает релиз 1.4.0 и отчасти 2.0.0, 2.1.0 (а также определенные подробности, которые не упоминались в прошлой статье). Здесь будут подробно объясняться моменты в том порядке, в котором они появлялись в проекте.

Также изначально планировалось в этой же статье рассказать про Go Wails установщик, но тогда она получается слишком большой, поэтому про это будет в следующей части.

## Сборка amnezia контейнера для клиента

Так как amnezia не предоставляет готовые cli решения для клиента, для сервера пришлось собирать все самостоятельно. Ниже будут файлы с пояснениями

**docker-compose.local.yaml:**
```yaml
amnezia-client:
    build:
      context: .
      dockerfile: Dockerfile.amnezia # dockerfile в котором билдится образ
    container_name: amnezia-client
    restart: unless-stopped
    cap_add: # выдаем контейнеру конкретные привилегии Linux ядра, вместо того чтобы выдавать полные root права
      - NET_ADMIN # позволяет контейнеру управлять сетевыми настройками хоста (создание интерфейсов, настройка таблиц маршрутизации, настраивать правила фильтрации трафика)
      - SYS_MODULE # дает возможность загружать и выгружать модули ядра Linux на хостовой системе (для работы Ansible заранее скачивает нужные модули ядра amnezia: https://github.com/amnezia-vpn/amneziawg-linux-kernel-module)
    privileged: true # дает возможность получать root права
    devices:
      - /dev/net/tun:/dev/net/tun # специальный виртуальный файл в Linux. Используется программами для создания соединений. Вместо отправки данных через физический кабель, программы отправляют их в этот файл, где они шифруются
    volumes:
      - ./apps-data/amnezia:/config # прокидываем файл awg0.conf
      - /lib/modules:/lib/modules:ro # прокидываем модули ядра
    logging: *default-logging
    network_mode: "host" # контейнер использует сетевой стек компьютера напрямую
    healthcheck:
      test: ["CMD-SHELL", "awg show | grep interface || exit 1"]
      interval: 30s
      timeout: 5s
      retries: 3
      start_period: 10s
```

**Dockerfile.amnezia:**
```dockerfile
FROM golang:1.24.5 AS builder # первый этап сборки, используем go для установки amnezia-go (нужен на всякий случай, если что-то случится с модулем ядра, так как он запускается полностью самостоятельно)
ARG TARGETARCH # переменные сборки, docker автоматически подставляет туда архитектуру процессора
ARG TARGETVARIANT

WORKDIR /amneziawg-go # рабочая директория

RUN git clone https://github.com/amnezia-vpn/amneziawg-go.git . --depth=1 \
    && go mod download \ # после клонирования качаем зависимости
    && go mod verify \
    && CGO_ENABLED=0 GOOS=linux GOARCH=$TARGETARCH GOARM=${TARGETVARIANT#v} \ # Отключает использование библиотек Си. Также указывает компилятору, под какую целевую ОС и архитектуру процессора нужно собрать файл
       go build -ldflags '-s -w' -v -o amneziawg-go # компилируем бинарник. Флаг -ldflags '-s -w' удаляет из готового файла отладочную информацию.

FROM alpine:3.22.1 # собираем финальный образ

ARG TARGETARCH
ARG AWGTOOLS_RELEASE=1.0.20250706 #версия

RUN apk --no-cache add \
    iproute2 \
    iptables \
    bash \
    openresolv \
    dumb-init \
    && apk --no-cache add --virtual .build-deps \
    dpkg \
    wget \
    unzip \
    git \
    build-base \
    linux-headers # устанавливаем зависимости

RUN git clone https://github.com/amnezia-vpn/amneziawg-tools.git /amneziawg-tools --depth=1 \ # клонируем консольные утилиты
    && cd /amneziawg-tools/src \
    && make \ # компилируем утилиты из исходного кода
    && make install \
    && ln -s /usr/local/bin/awg /usr/bin/wg \ # создаем ярлыки (если будет вызываться wg, перенаправим на awg)
    && ln -s /usr/local/bin/awg-quick /usr/bin/wg-quick \
    && cd / \
    && rm -rf /amneziawg-tools \ # удаляем исходники
    && apk del .build-deps # удаляем компиляторы, которые ставили временно

COPY --from=builder /amneziawg-go/amneziawg-go /usr/bin/ # копируем бинарник go версии с предыдущего этапа сборки

COPY scripts/amnezia-client_init.sh /init.sh
RUN chmod +x /init.sh

ENTRYPOINT ["/usr/bin/dumb-init", "/init.sh"] # запускаем скрипт. dumb-init встает на место главного процесса PID 1 и запускает скрипт, слушает команды Docker и транслирует внутрь клиента. Dumb-init нужен, так как bash скрипты сами по себе не умеют управлять дочерними процессами и обрабатывать системные сигналы.
```

**amnezia-client_init.sh:**
```bash
#!/bin/bash
find /etc/amnezia/amneziawg -mindepth 1 -delete # удаляем все файлы в директории, гарантирует чистый старт

COUNTER=0 # для проверки удалось ли что-нибудь найти
for s in $(find /config -name "*.conf") # ищем конфиги
do
  if test -f ${s} # убеждаемся что это файл
  then
    COUNTER=$(( COUNTER + 1 ))
    basename=$(basename ${s}) # извлекаем имя файла
    name=${basename%.conf}
    echo awg interface "${name}" will be created from config file "${basename}"
    cp ${s} /etc/amnezia/amneziawg/${name}.conf # копируем себе
    chmod 600 /etc/amnezia/amneziawg/${name}.conf # выдаем права: чтение и запись только для root
    awg-quick up ${name} # главная команда, которая создаем виртуальный сетевой интерфейс с именем файла
  fi
done

if [[ $COUNTER -lt 1 ]]
then
  echo "There are no config files in the /config folder"
fi

sleep infinity # контейнер живет до тех пор, пока работает главный процесс PID 1
```

## Появление Gitea + настройки в Nginx

**docker-compose.local.yaml:**
```yaml
 gitea:
    image: gitea/gitea:1.26
    container_name: gitea
    logging: *default-logging
    environment:
      - USER_UID=1000
      - USER_GID=1000
      - GITEA__server__ROOT_URL=https://${SERVER_NAME}/gitea/ # эти переменные служат для генерации файла конфига .ini (формат GITEA__SECTION__KEY)
      - GITEA__server__DISABLE_SSH=true # отключам ssh чтобы не пробрасывать через nginx, так как работает в vpn сети, можно спокойно пользоваться https
      - GITEA__database__DB_TYPE=sqlite3
      - GITEA__database__PATH=/data/gitea/gitea.db
      - GITEA__security__INSTALL_LOCK=true # отключаем страницу первоначальной установки, сразу запуская gitea в рабочем режиме
    volumes:
      - ./apps-data/gitea:/data
      - /etc/timezone:/etc/timezone:ro
      - /etc/localtime:/etc/localtime:ro
    expose:
      - 3000 # даем доступ только в docker сети для nginx
    restart: unless-stopped
```

**nginx.remote.conf.template:** (location который перенаправляет трафик на локальные сервисы)
```nginx
location ~* ^/(navidrome|audiobookshelf|nextcloud|gitea|vaultwarden|grafana|portainer|borg-ui|v2) {  
	# перенаправляем запрос на Nginx локального сервера внутри vpn
	# оригинальный путь (URI) передается автоматически, что позволит локальному Nginx распределить запросы по сервисам
	proxy_pass http://10.8.0.2:80; 

	proxy_http_version 1.1; 
	# пробрасываем заголовки для поддержки WebSockets (необходимо для Vaultwarden, Portainer и др.)
	proxy_set_header Upgrade $http_upgrade; 
	proxy_set_header Connection $connection_upgrade; 

	# передаем оригинальные данные клиента, чтобы конечные сервисы знали реальный IP и домен
	proxy_set_header Host $host; 
	proxy_set_header X-Real-IP $remote_addr; 
	proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for; 
	proxy_set_header X-Forwarded-Host $host; 

	# сообщаем сервисам, что внешнее соединение защищено https (предотвращает проблемы с безопасностью и редиректами)
	proxy_set_header X-Forwarded-Proto https; 

	# отключаем буферизацию, чтобы данные передавались мгновенно в реальном времени 
	# это критично для стриминга медиа и загрузки больших файлов через Nextcloud.
	proxy_buffering off; 
	proxy_request_buffering off; 

	# увеличиваем таймауты до 1 часа, чтобы не рвались веб-сокеты и долгие загрузки файлов
	proxy_read_timeout 3600; 
	keepalive_timeout 3600; 
}
```

Также вы могли заметить v2 путь. Он нужен для того, чтобы использовать gitea registry, так как docker push по спецификации всегда добавляет в путь /v2/, а так как gitea в моем случае находится не в корне домена, без этого все ломается.

К примеру: docker делает запрос и получается путь https://server.name/v2/, при этом gitea находится на https://server.name/gitea/. Соответственно нужно просто “обмануть” docker чтобы /v2/ шел на локальный сервер, и там уже шел в gitea, как если бы docker обратился по /gitea/.

**nginx.local.conf.template:** (location для gitea на локальном сервере)
```nginx
# ведем оба пути в ту же самую gitea, только тут есть небольшой нюанс, который позволяет всему этому работать
 location /gitea/ { 
        # слэш на конце "http://gitea:3000/" отрезает префикс "/gitea/" из запроса.
        # например, запрос к "/gitea/explore" уйдет в контейнер как "/explore".
        proxy_pass http://gitea:3000/; 

        proxy_set_header Host $http_host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto https;
        proxy_buffering off;

        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection $connection_upgrade;
    }

    location /v2 {
        # слэша на конце нет, поэтому Nginx проксирует путь "/v2" без изменений.
        # запрос "/v2/token" уйдет в контейнер именно как "/v2/token", где его обработает реестр Gitea.
        proxy_pass http://gitea:3000; 

        proxy_set_header Host $http_host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto https;

        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection $connection_upgrade;
    }
```

## Redis для Nextcloud

Здесь кратко пройдусь по финальному виду docker-compose файла для Nextcloud с описаниями

**docker-compose.local.yaml:**
```yaml
nextcloud-db:
    image: postgres:18
    container_name: nextcloud-db
    logging: *default-logging
    restart: unless-stopped
    volumes:
      - ./apps-data/postgres/nextcloud:/var/lib/postgresql
    environment:
      - POSTGRES_DB=${POSTGRES_DB_NEXTCLOUD}
      - POSTGRES_USER=${POSTGRES_USER}
      - POSTGRES_PASSWORD=${POSTGRES_PASSWORD}
    expose:
      - 5432
    healthcheck:
      # проверка здоровья БД: pg_isready проверяет, готов ли сервер принимать подключения
      test: ["CMD-SHELL", "pg_isready -U $$POSTGRES_USER -d $$POSTGRES_DB"]
      interval: 5s
      timeout: 5s
      retries: 5
    deploy:
      resources:
        limits:
          memory: 1G

  nextcloud-redis:
    image: redis:8.8.0
    container_name: nextcloud-redis
    restart: unless-stopped
    # устанавливаем пароль прямо в команде запуска сервера для безопасности
    command: redis-server --requirepass ${NEXTCLOUD_REDIS_PASS}
    logging: *default-logging
    deploy:
      resources:
        limits:
          memory: 1G

  nextcloud:
    image: nextcloud:34.0.0-apache
    container_name: nextcloud
    logging: *default-logging
    restart: unless-stopped
    expose:
      - 80
    depends_on:
      # запуск Nextcloud начнется только тогда, когда база пройдет проверку здоровья (healthcheck)
      nextcloud-db:
        condition: service_healthy
      nextcloud-redis:
        condition: service_started
    environment:
      - POSTGRES_HOST=nextcloud-db
      # OVERWRITEWEBROOT важен, так как Nextcloud запущен не в корне домена, а в /nextcloud
      - OVERWRITEWEBROOT=/nextcloud
      - OVERWRITEPROTOCOL=https
      # список доверенных прокси (включая ваш VPN 10.8.0.0/24), чтобы Nextcloud 
      # корректно определял IP клиента через заголовки X-Forwarded-For
      - TRUSTED_PROXIES=172.16.0.0/12 10.8.0.0/24
      # настройка кэширования: APCu для локального кэша, Redis для блокировки файлов
      - 'NC_memcache.local=\OC\Memcache\APCu'
      - 'NC_memcache.locking=\OC\Memcache\Redis'
    volumes:
      - ./apps-data/nextcloud/html:/var/www/html
      - /home/${LOCAL_USER}/PersonalData/NextcloudData:/var/www/html/data
    deploy:
      resources:
        limits:
          memory: 3G
    healthcheck:
      # проверка: существует ли конфиг и работает ли PHP-утилита OCC (Nextcloud CLI)
      test: ["CMD-SHELL", "test -f /var/www/html/config/config.php && php occ status"]
      interval: 1m30s
      timeout: 30s
      retries: 5
      start_period: 30s

  nextcloud-cron:
    image: nextcloud:34.0.0-apache
    container_name: nextcloud-cron
    restart: unless-stopped
    # этот контейнер только для запуска фоновых задач (очистка кэша, превью, индексация)
    # это разгружает основной процесс веб-сервера
    entrypoint: /cron.sh
    logging: *default-logging
    depends_on:
      nextcloud-db:
        condition: service_healthy
      nextcloud:
        condition: service_healthy
    deploy:
      resources:
        limits:
          memory: 512M
```

## Исправления ошибок

### Сборка модулей для 7.1.3-arch1-2

Проблема заключалась в том, что при обновлении ядра Linux одна из внутренних функций изменила свое название, из-за чего модуль amneziawg перестал компилироваться (он все еще пытался использовать старый вызов). Стоит отметить, что из-за быстрого обновления ядра в Arch Linux подобные ситуации с внешними модулями случаются, и проект не всегда может корректно собраться автоматически без вмешательства.

Чтобы это исправить я добавил в Ansible такой шаг:
```yaml
- name: "[AmneziaWG] Arch: исправление совместимости с ядром 7.x+ (ipv6_stub)"
  block:
    - name: "[AmneziaWG] Arch: проверяем наличие исходников в /usr/src"
      ansible.builtin.stat:
        path: /usr/src/amneziawg-1.0.0/socket.c
      register: amneziawg_source # записываем результат есть ли исходники

    - name: "[AmneziaWG] Arch: замена ipv6_stub на ip6_dst_lookup_flow"
      ansible.builtin.replace:
        path: /usr/src/amneziawg-1.0.0/socket.c
        regexp: 'ipv6_stub->ipv6_dst_lookup_flow' # находим функцию и меняем название
        replace: 'ip6_dst_lookup_flow'
      register: patch_result # пишем удалось ли перезаписать
      when: amneziawg_source.stat.exists # выполняется только если найдены исходники

    - name: "[AmneziaWG] Arch: принудительная пересборка модуля после патча"
      ansible.builtin.shell: |
        dkms remove amneziawg/1.0.0 --all || true # удаляем загруженный в оперативку модуль
        dkms add -m amneziawg -v 1.0.0 || true # добавляем обратно
        dkms build -m amneziawg -v 1.0.0 # билдим
        dkms install -m amneziawg -v 1.0.0 # устанавливаем
      become: true # используем root
      when: patch_result.changed
  when: ansible_facts['os_family'] == "Archlinux"
```

Если что dkms - это фреймворк, который позволяет автоматически пересобирать модули ядра при каждом обновлении самого ядра в системе. Ansible-скрипт в данном случае просто патчит код и инициирует пересборку.

### Nginx и Loki не запускаются после перезагрузки сервера

Проблема заключалась в том, что в docker-compose файле Nginx и Loki были привязаны к IP-адресу VPN-туннеля (10.8.0.2). При старте системы Docker пытался запустить контейнеры раньше, чем сетевой интерфейс VPN успевал подняться. Из-за отсутствия нужного IP на хосте контейнеры аварийно завершались.

Чтобы это решить я добавил в Ansible такой шаг:
```yaml
- name: "[Net] Разрешить привязку к несуществующим IP (ip_nonlocal_bind)"
  ansible.posix.sysctl: # используем sysctl
    name: net.ipv4.ip_nonlocal_bind
    value: '1' # ставим значение 1 (раньше было 0)
    state: present # Убеждаемся, что параметр установлен 
    reload: yes # Применяем изменения параметров ядра немедленно
```

## Заключение

Подробно рассказал про все изменения, которые были сделаны и про исправления ошибок, а также про то, что упустил в предыдущей части. Следующая статья будет полностью посвящена Go Wails установщику, который автоматизирует установку и разворачивание сервисов на серверах.