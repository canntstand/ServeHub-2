# Habitica self-hosted: полный гайд по развёртыванию, настройке Nginx и docker-compose

## Предыстория

Я решил внедрить и трекер задач, так как это был единственный инструмент, который выбивался из моего привычного workflow и существовал полностью отдельно. Из-за этого список ежедневных дел мне приходилось копировать в Obsidian, чтобы в случае удаления старого трекера не бояться, что мои планы пропадут.

После недолгих поисков меня привлекли два варианта: Habitica и Vikunja. До сих пор оба кажутся мне отличными решениями, но я решил остановиться на Habitica. Геймификация в ней меня действительно зацепила, и, пользуясь этим подходом последние несколько дней, я не жалею. Возможно, Habitica в какой-то степени помогает справиться с известной проблемой: когда появляется слишком много дел, в обычные игры играть не хочется совсем, хотя у меня уже собран список из более чем 50 тайтлов, которые я хочу пройти.

Также стоит сразу отметить, что Habitica — весьма специфический to-do инструмент, и многим он может не подойти. Перед тем как разворачивать его самостоятельно, советую просто скачать официальное приложение и некоторое время попользоваться им через серверы разработчиков, чтобы понять, подходит оно вам или нет.

И последнее, о чём хотелось бы упомянуть перед переходом к настройке, — это трекер HabitNow, которым я пользовался раньше. Если честно, большого практического смысла в этом блоке нет, просто я считаю данный трекер лучшим из тех, что использовал (если вам не нужна геймификация). Информации о нём мало, поэтому хочется привлечь к нему внимание. Если вам не нравится Habitica или self-hosted решения, обязательно попробуйте HabitNow.

## Настройка

Если вы планируете использовать этот гайд при развёртывании Habitica, советую также ознакомиться с информацией в официальном форке для self-hosted. Всё, что я делаю здесь — это скорее кастомные решения, которые мне пришлось применить для корректной работы в моих условиях. В вашем случае ситуация может отличаться, и стандартного руководства вполне может хватить: https://github.com/awinterstein/habitica

Для развёртывания я использовал docker-compose, поэтому показывать процесс буду на его примере. Мне кажется, это самый простой и автоматизированный вариант.

Также в своей конфигурации я использовал Nginx в качестве reverse proxy для проксирования трафика в Habitica.

**docker-compose.local.yaml:**
```yaml
  habitica:
    image: docker.io/awinterstein/habitica-server:5.48.6 # Используем образ от awinterstein, адаптированный для self-hosted
    container_name: habitica-server
    restart: unless-stopped
    logging: *default-logging
    depends_on:
      habitica-db:
        condition: service_healthy
    environment:
      - NODE_DB_URI=mongodb://habitica-db/habitica # Подключение к базе данных
      - BASE_URL=http://habitica.${SERVER_NAME} # Использование поддомена (запустить Habitica в subpath не удалось). При работе через reverse proxy используйте http во избежание бесконечного редиректа.
      - INVITE_ONLY=false # Открытая регистрация. Рекомендуется отключить (true) после создания пользователя, если доступ не ограничен на уровне VPN как в моем случае.
    networks:
      default:
        aliases:
          - server # Алиас необходим, так как встроенный Caddy в клиенте жестко завязан на хост "server"
          - habitica-server
    expose: # Лишние порты наружу не открываем
      - 3000
    deploy:
      resources:
        limits:
          memory: 1G

  habitica-client:
    image: docker.io/awinterstein/habitica-client:5.48.6 # Веб-интерфейс для доступа через браузер (помимо приложения)
    container_name: habitica-client
    restart: unless-stopped
    logging: *default-logging
    depends_on:
      habitica:
        condition: service_started
    expose:
      - 80
    deploy:
      resources:
        limits:
          memory: 512M

  habitica-db:
    image: docker.io/mongo:7.0 # Используем v7.0 из-за совместимости с новыми ядрами Linux (на Arch Linux), но вы можете выбрать другую версию
    container_name: habitica-db
    restart: unless-stopped
    hostname: habitica-db
    command: ["--replSet", "rs", "--bind_ip_all", "--port", "27017"] # Включение ReplicaSet (требуется для поддержки транзакций в Node.js)
    logging: *default-logging
    healthcheck:
      # Инициализация ReplicaSet в одну строку при первом запуске
      test: echo "try { rs.status() } catch (err) { rs.initiate() }" | mongosh --port 27017 --quiet
      interval: 10s
      timeout: 30s
      start_period: 5s
      start_interval: 1s
      retries: 30
    volumes:
      - ./apps-data/habitica-db:/data/db:rw # Монтирование в отдельную папку для удобного переноса и бэкапов
    deploy:
      resources:
        limits:
          memory: 1G
```

Теперь перейдём к конфигурации Nginx. У меня используется каскад из двух Nginx (внешний проксирует трафик на внутренний, который затем направляет его в Habitica). Я покажу оба конфига. Если вы планируете использовать только один Nginx, то логику SSL нужно будет перенести из `nginx.remote` в `nginx.local`.

**nginx.local.conf.template:** (часть, относящаяся к Habitica)
```nginx
server {
    listen 80; # Слушаем порт 80, так как SSL-терминация происходит на внешнем Nginx, а далее трафик идет по защищенному VPN
    server_name habitica.${SERVER_NAME}; # Поддомен для Habitica

    location / { # Браузерный клиент (для использования с ПК)
        proxy_pass http://habitica-client:80/; 
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        # Передаем исходный протокол HTTPS, чтобы Node.js внутри контейнера корректно обрабатывал ссылки
        proxy_set_header X-Forwarded-Proto https; 
    }

    location /api/ { # API для работы мобильного приложения
        proxy_pass http://habitica:3000/api/;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto https;
    }
}
```

**nginx.remote.conf.template:** (внешний Nginx, который проксирует в nginx.local)
```nginx
server {
    listen 443 ssl; # Здесь обрабатываются SSL-сертификаты. Если каскад не требуется, этот блок можно объединить с nginx.local
    server_name habitica.${SERVER_NAME};

    ssl_certificate /etc/letsencrypt/live/${SERVER_NAME}/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/${SERVER_NAME}/privkey.pem;

    client_max_body_size 2G; # Лимит на размер загружаемых файлов с запасом
    add_header Strict-Transport-Security "max-age=15552000; includeSubDomains; preload" always;

    # Доверенные IP-диапазоны. Если VPN не используется, эти строки вместе с allow/deny можно убрать
    set_real_ip_from 172.16.0.0/12; 
    set_real_ip_from 10.0.0.0/8;
    set_real_ip_from 127.0.0.1;
    real_ip_recursive on;

    allow 127.0.0.1;
    allow 172.16.0.0/12;
    allow 10.8.0.0/24;
    deny all;

    location / {
        proxy_pass http://10.8.0.2:80; # Проксируем на внутренний Nginx в цепочке
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection $connection_upgrade; # Для работы вебсокетов (требует наличия map $http_upgrade $connection_upgrade в nginx.conf)
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto https;
    }
}
```

## Заключение

В принципе, это всё, что я хотел показать. Надеюсь, что кому-то мои решения помогут сэкономить время, так как с Nginx у меня сначала возникли небольшие трудности из-за конфликтов со встроенным Caddy, но в итоге всё удалось успешно настроить.