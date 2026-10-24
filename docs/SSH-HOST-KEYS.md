# Проверка SSH host keys перед развёртыванием

ServeHub больше не принимает SSH host key автоматически. Ansible использует
`StrictHostKeyChecking yes` и отдельный файл
`/root/.ssh/known_hosts.verified` в постоянном Docker volume `ssh-data`.
Старый `known_hosts`, созданный режимом `accept-new`, не используется.

## 1. Получить fingerprint независимо

Для каждого адреса из `ansible/vars/secrets.yml` (публичный IP VPS и адрес
Workload, используемый Ansible) задайте переменную `HOST_IP` на машине, где
запускается ServeHub. Команда сохраняет candidate host key в постоянном SSH volume
и печатает его fingerprint:

```sh
HOST_IP="адрес-сервера"

docker compose -f docker-compose.ansible.yaml run --rm --no-deps \
  --entrypoint /bin/sh ansible -c \
  'ssh-keyscan -H -t ed25519 "$1" > /root/.ssh/hostkey-candidate && ssh-keygen -lf /root/.ssh/hostkey-candidate' \
  sh "$HOST_IP"
```

Сверьте fingerprint с данными из независимого доверенного источника: веб-консоли
провайдера VPS или локальной консоли Workload. Сам по себе результат
`ssh-keyscan` не подтверждает подлинность ключа.

## 2. Закрепить именно проверенный ключ

Только после успешной независимой проверки fingerprint выполните:

```sh
docker compose -f docker-compose.ansible.yaml run --rm --no-deps \
  --entrypoint /bin/sh ansible -c \
  'test -s /root/.ssh/hostkey-candidate && cat /root/.ssh/hostkey-candidate >> /root/.ssh/known_hosts.verified && chmod 600 /root/.ssh/known_hosts.verified' \
```

Повторите шаги для второго сервера. Candidate перезаписывается при получении
следующего ключа, а проверенные записи накапливаются в `known_hosts.verified`.
После закрепления ключей запускайте `scripts/manage_deploy.sh` обычным способом.

Если сервер переустановлен или ключ изменился, не обновляйте запись автоматически:
сначала проверьте новый fingerprint через консоль провайдера и только после этого
обновите запись. Ожидаемое поведение при несовпадении ключа — отказ SSH-соединения.
