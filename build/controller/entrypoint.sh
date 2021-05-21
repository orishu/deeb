#!/bin/sh

cp /var/secrets/id_rsa id_rsa
chown app:app id_rsa

sudo --preserve-env -u app ./controller -bootstrap
