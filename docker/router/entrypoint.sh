#!/bin/sh
set -eu

mkdir -p /run/dslab

sysctl -w net.ipv4.ip_forward=1 >/dev/null 2>&1 || true

lan_iface="$(ip -o -4 addr show | awk -v ip="$LAN_IP" '$4 ~ ("^" ip "/") { print $2; exit }')"
backbone_iface="$(ip -o -4 addr show | awk -v ip="$BACKBONE_IP" '$4 ~ ("^" ip "/") { print $2; exit }')"

if [ -z "$lan_iface" ] || [ -z "$backbone_iface" ]; then
  echo "failed to discover router interfaces" >&2
  exit 1
fi

printf '%s\n' "$lan_iface" > /run/dslab/lan_iface
printf '%s\n' "$backbone_iface" > /run/dslab/backbone_iface

IFS=','
for route in ${REMOTE_ROUTES:-}; do
  [ -z "$route" ] && continue
  subnet="${route%%:*}"
  gateway="${route##*:}"
  ip route replace "$subnet" via "$gateway" dev "$backbone_iface"
done
unset IFS

exec tail -f /dev/null

