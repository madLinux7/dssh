#!/usr/bin/env bash
#set -euo pipefail

if [ -z "$1" ] || [ "$1" -eq 0 ]; then
  echo "Number of records required"
  exit 1
fi

DSSH="./dssh"

adjectives=(alpha bravo charlie delta echo foxtrot golf hotel india juliet
            kilo lima mike november oscar papa quebec romeo sierra tango
            uniform victor whiskey xray yankee zulu swift rusty dark bright
            cold warm deep flat sharp bold calm brave prime lunar solar
            frost blaze storm cloud ember coral jade onyx pearl ruby sage)

nouns=(server node gateway proxy relay bridge switch router beacon tower
       forge vault forge cluster shard replica edge core mesh link
       bastion citadel bunker haven nexus orbit pulse spark drift anchor)

users=(root admin deploy ops sysadmin devops ci jenkins ansible terraform
       ubuntu ec2-user centos alpine docker kube prometheus grafana vault
       consul nomad)

for i in $(seq 1 $1); do
    adj=${adjectives[$((RANDOM % ${#adjectives[@]}))]}
    noun=${nouns[$((RANDOM % ${#nouns[@]}))]}
    name="${adj}-${noun}-$(printf '%03d' "$i")"

    user=${users[$((RANDOM % ${#users[@]}))]}

    octet2=$((RANDOM % 256))
    octet3=$((RANDOM % 256))
    octet4=$((RANDOM % 254 + 1))
    host="10.${octet2}.${octet3}.${octet4}"

    port=$((RANDOM % 5 == 0 ? 2222 : 22))

    $DSSH add -p "$port" "$name" "${user}@${host}"
done

echo "Inserted $1 mockup connections."
