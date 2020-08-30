#!/bin/sh

cp /var/secrets/id_rsa.pub /home/mysql/.ssh/id_rsa.pub
cp /var/secrets/id_rsa.pub /home/mysql/.ssh/authorized_keys
cp /var/secrets/id_rsa /home/mysql/.ssh/id_rsa
chown mysql:mysql /home/mysql/.ssh/*

exec /usr/sbin/sshd  -e -D