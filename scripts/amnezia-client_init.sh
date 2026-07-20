#!/bin/bash
find /etc/amnezia/amneziawg -mindepth 1 -delete

COUNTER=0
for s in $(find /config -name "*.conf")
do
  if test -f ${s}
  then
    COUNTER=$(( COUNTER + 1 ))
    basename=$(basename ${s})
    name=${basename%.conf}
    echo awg interface "${name}" will be created from config file "${basename}"
    cp ${s} /etc/amnezia/amneziawg/${name}.conf
    chmod 600 /etc/amnezia/amneziawg/${name}.conf
    awg-quick up ${name}
  fi
done

if [[ $COUNTER -lt 1 ]]
then
  echo "There are no config files in the /config folder"
fi

if [[ $COUNTER -gt 0 ]]
then
  echo "Setting DNS to 10.8.0.1..."
  echo "nameserver 10.8.0.1" | sudo tee /etc/resolv.conf > /dev/null
fi

sleep infinity
