FROM percona:ps-8.0

USER root

RUN mkdir /home/mysql && chown mysql:mysql /home/mysql && usermod -m -d /home/mysql -s /bin/bash mysql
USER mysql
RUN mkdir /home/mysql/.ssh
COPY ssh_config /home/mysql/.ssh/config
USER root

RUN yum -y install percona-xtrabackup-80

RUN \
  yum -y install openssh-clients openssh-server && \
  yum -y clean all && \
  touch /run/utmp && \
  chmod u+s /usr/bin/ping && \
  echo "root:root" | chpasswd

RUN /usr/bin/ssh-keygen -A

COPY entrypoint.sh /

ENTRYPOINT /entrypoint.sh

EXPOSE 22
