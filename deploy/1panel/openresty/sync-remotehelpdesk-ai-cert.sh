#!/usr/bin/env bash
set -euo pipefail

cert_name="remotehelpdesk-ai.online"
source_dir="/etc/letsencrypt/live/${cert_name}"
target_dir="/opt/1panel/apps/openresty/openresty/conf/ssl/${cert_name}"
openresty_container="1Panel-openresty-NryP"

install -d -m 700 "$target_dir"
install -m 644 "${source_dir}/fullchain.pem" "${target_dir}/fullchain.pem"
install -m 600 "${source_dir}/privkey.pem" "${target_dir}/privkey.pem"

if ! docker exec "$openresty_container" openresty -t >/dev/null 2>&1; then
    echo "OpenResty configuration validation failed" >&2
    exit 1
fi

docker exec "$openresty_container" openresty -s reload
