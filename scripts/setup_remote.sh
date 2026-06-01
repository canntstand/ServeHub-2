#!/bin/bash

chmod +x scripts/amneziawg-install.sh
chmod +x scripts/amneziawg-web.sh

sudo AUTO_INSTALL=y scripts/amneziawg-install.sh

sudo scripts/amneziawg-install.sh --add-client home_pc

sudo cp ~/awg0-client-home_pc.conf ./amnezia-data/client_home.conf

sudo scripts/amneziawg-web.sh install --install-rust