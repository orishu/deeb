FROM ubuntu

USER root

RUN groupadd -g 1001 mysql
RUN useradd -g mysql -m -u 1001 -s /bin/bash mysql
 
#RUN mkdir /home/mysql && chown mysql:mysql /home/mysql && usermod -m -d /home/mysql -s /bin/bash mysql
USER mysql
RUN mkdir /home/mysql/.ssh
COPY ssh_config /home/mysql/.ssh/config
USER root

RUN apt update && apt install -y curl lz4 openssh-client openssh-server net-tools iproute2 && \
  curl -O https://repo.percona.com/apt/percona-release_latest.generic_all.deb && \
  apt install -y gnupg2 lsb-release ./percona-release_latest.generic_all.deb && \
  apt update && \
  percona-release setup pxb-80 && \
  apt install -y percona-xtrabackup-80 && \
  touch /run/utmp && \
  echo "root:root" | chpasswd

RUN /usr/bin/ssh-keygen -A

COPY entrypoint.sh /

ENTRYPOINT ["/entrypoint.sh"]

EXPOSE 22

