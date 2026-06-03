echo "Проверка автозапуска Docker..."
if ! systemctl is-enabled --quiet docker; then
    echo "Включаю автозапуск Docker..."
    sudo systemctl enable docker
fi

sudo systemctl start docker

if [ -f .env ]; then
    export $(grep -v '^#' .env | xargs)
else
    echo "ОШИБКА: Файл .env не найден!"
    exit 1
fi

echo "Настройка сертификатов..."
if [ ! -d "./certbot-dns-webnames" ]; then
    git clone https://github.com/regtime-ltd/certbot-dns-webnames.git ./certbot-dns-webnames
fi
curl -s -k "https://www.webnames.ru/scripts/json_domain_zone_manager.pl?action=get_config_certbot&domain=${SYNAPSE_SERVER_NAME}&apikey=${WEBNAMES_APIKEY}" -o ./certbot-dns-webnames/config.sh
chmod +x ./certbot-dns-webnames/*.sh

CERT_DIR="./certs/live/${SYNAPSE_SERVER_NAME}"
if [ ! -f "${CERT_DIR}/fullchain.pem" ]; then
    mkdir -p "${CERT_DIR}"
    openssl req -x509 -nodes -days 1 -newkey rsa:2048 -keyout "${CERT_DIR}/privkey.pem" -out "${CERT_DIR}/fullchain.pem" -subj "/CN=localhost"
    NEED_REAL_CERT=true
else
    NEED_REAL_CERT=false
fi

if [ "$NEED_REAL_CERT" = true ]; then
    echo "Получение реального сертификата..."
    sudo docker compose -f docker-compose.remote.yaml build certbot
    rm -rf "${CERT_DIR:?}"/*
    sudo docker compose -f docker-compose.remote.yaml run --rm certbot
    sudo docker compose -f docker-compose.remote.yaml exec nginx nginx -s reload
fi

echo "Применение системных настроек для работы VPN..."
echo "net.ipv4.ip_forward=1" | sudo tee -a /etc/sysctl.conf
echo "net.ipv4.conf.all.src_valid_mark=1" | sudo tee -a /etc/sysctl.conf
echo "net.ipv6.conf.all.disable_ipv6=0" | sudo tee -a /etc/sysctl.conf
echo "net.ipv6.conf.all.forwarding=1" | sudo tee -a /etc/sysctl.conf
echo "net.ipv6.conf.default.forwarding=1" | sudo tee -a /etc/sysctl.conf
sudo sysctl -p
sudo iptables -A FORWARD -i wg0 -j ACCEPT
sudo iptables -A FORWARD -o wg0 -j ACCEPT
sudo iptables -t nat -A POSTROUTING -s 10.8.0.0/24 -o eth0 -j MASQUERADE

sudo docker compose -f docker-compose.remote.yaml up -d