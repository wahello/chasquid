#!/bin/bash

set -e
. $(dirname ${0})/../util/lib.sh

init

rm -rf .data-A .data-B .data-C .mail .logs

# 3 servers:
# A - listens on :1025, hosts srv-A
# B - listens on :2015, hosts srv-B
# C - listens on :3015, hosts srv-C
#
# B and C are normal servers.
# A will use B as a smarthost.
#
# We will send an email from A to C, and expect it to go through B.

mkdir -p .certs
for i in A B C; do
		CONFDIR=${i} generate_certs_for srv-${i}
		CONFDIR=${i} add_user user${i}@srv-${i} user${i}
		mkdir -p .logs-${i}
		cp ${i}/certs/srv-${i}/fullchain.pem .certs/cert-${i}.pem
done

# Make the servers trust each other.
export SSL_CERT_DIR="$PWD/.certs/"

chasquid -v=2 --logfile=.logs-A/chasquid.log --config_dir=A &
chasquid -v=2 --logfile=.logs-B/chasquid.log --config_dir=B \
	--testing__outgoing_smtp_port=3025 &
chasquid -v=2 --logfile=.logs-C/chasquid.log --config_dir=C \
	--testing__outgoing_smtp_port=2025 &

wait_until_ready 1025
wait_until_ready 2025
wait_until_ready 3025

# Use A to send to C, and wait for delivery.
run_msmtp userC@srv-c < content
wait_for_file .mail/userc@srv-c
mail_diff content .mail/userc@srv-c

# Check that it went through B.
if ! grep -q "from=userA@srv-a to=userC@srv-c sent" .logs/mail_log-B; then
	fail "can't find record of delivery on the smarthost B"
fi

success
