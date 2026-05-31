#!/bin/bash

DATA_DIR="./amnezia-data"
CONFIG_FILE="$DATA_DIR/client_server.conf"

if [ -f .env ]; then
    export $(grep -v '^#' .env | xargs)
fi
PUBLIC_IP=${SERVER_IP:-$(curl -s ifconfig.me)}
VPN_PORT=39311

if [ -f "$CONFIG_FILE" ]; then
    echo "Конфигурация VPN уже существует. Пропускаем генерацию."
    exit 0
fi

echo "Инициализация структуры AmneziaWG..."
mkdir -p "$DATA_DIR"

gen_key() {
    docker run --rm ghcr.io/amnezia-vpn/amneziawg-go:latest wg genkey
}
get_pub() {
    echo "$1" | docker run --rm -i ghcr.io/amnezia-vpn/amneziawg-go:latest wg pubkey
}
gen_psk() {
    docker run --rm ghcr.io/amnezia-vpn/amneziawg-go:latest wg genpsk
}

echo "Генерация криптографических ключей..."
SERVER_PRIV=$(gen_key)
SERVER_PUB=$(get_pub "$SERVER_PRIV")

HOME_PRIV=$(gen_key)
HOME_PUB=$(get_pub "$HOME_PRIV")
HOME_PSK=$(gen_psk)

echo "Сохранение ключей в отдельные файлы..."
echo "$SERVER_PRIV" > "$DATA_DIR/wireguard_server_private_key.key"
echo "$SERVER_PUB" > "$DATA_DIR/wireguard_server_public_key.key"
echo "$HOME_PSK" > "$DATA_DIR/wireguard_psk.key"

echo "Генерация уникальных параметров обфускации AmneziaWG..."
JC=$((RANDOM % 5 + 3))
JMIN=$((RANDOM % 30 + 10))
JMAX=$((RANDOM % 300 + 200))
S1=$((RANDOM % 40 + 15))
S2=$((RANDOM % 40 + 15))
S3=$((RANDOM % 40 + 15))
S4=$((RANDOM % 40 + 15))

H1=$(shuf -i 100000000-2000000000 -n 1)
H2=$(shuf -i 100000000-2000000000 -n 1)
H3=$(shuf -i 100000000-2000000000 -n 1)
H4=$(shuf -i 100000000-2000000000 -n 1)

cat <<EOF > "$CONFIG_FILE"
[Interface]
PrivateKey = $SERVER_PRIV
Address = 10.8.1.1/24
ListenPort = $VPN_PORT

Jc = $JC
Jmin = $JMIN
Jmax = $JMAX
S1 = $S1
S2 = $S2
S3 = $S3
S4 = $S4
H1 = $H1
H2 = $H2
H3 = $H3
H4 = $H4

[Peer]
PublicKey = $HOME_PUB
PresharedKey = $HOME_PSK
AllowedIPs = 10.8.1.2/32
EOF

cat <<EOF > "$DATA_DIR/client_home.conf"
[Interface]
PrivateKey = $HOME_PRIV
Address = 10.8.1.2/24

Jc = $JC
Jmin = $JMIN
Jmax = $JMAX
S1 = $S1
S2 = $S2
S3 = $S3
S4 = $S4
H1 = $H1
H2 = $H2
H3 = $H3
H4 = $H4

[Peer]
PublicKey = $SERVER_PUB
PresharedKey = $HOME_PSK
Endpoint = $PUBLIC_IP:$VPN_PORT
AllowedIPs = 10.8.1.0/24
PersistentKeepalive = 25
EOF

echo "Папка $DATA_DIR успешно подготовлена."
echo "Сгенерированы файлы: awg0.conf и client_home.conf"

chmod 600 "$DATA_DIR"/*.conf
chmod 600 "$DATA_DIR"/*.key

sudo docker compose -f docker-compose.remote.yaml up -d