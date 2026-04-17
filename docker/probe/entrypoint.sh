#!/bin/sh
set -eu

mkdir -p /run/dslab

lan_iface="$(ip -o -4 addr show | awk -v ip="$NODE_IP" '$4 ~ ("^" ip "/") { print $2; exit }')"

if [ -z "$lan_iface" ]; then
  echo "failed to discover probe interface" >&2
  exit 1
fi

printf '%s\n' "$lan_iface" > /run/dslab/lan_iface

IFS=','
for subnet in ${REMOTE_SUBNETS:-}; do
  [ -z "$subnet" ] && continue
  ip route replace "$subnet" via "$ROUTER_IP" dev "$lan_iface"
done
unset IFS

exec python /app/serve.py

