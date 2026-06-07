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
    iptables -A FORWARD -i ${name} -j ACCEPT
    iptables -A FORWARD -o ${name} -j ACCEPT
    iptables -A FORWARD -i ${name} -o ${name} -j ACCEPT
  fi
done

if [[ $COUNTER -lt 1 ]]
then
  echo "There are no config files in the /config folder"
fi

tail -f /dev/null