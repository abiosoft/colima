ARG ARCH="x86_64"

FROM ubuntu-layer-${ARCH}

# use root user for root actions
USER root

# change SSH port to avoid clash
RUN echo "Port 23" >> /etc/ssh/sshd_config

# args
ARG NONROOT_USER="user"
ARG UID=501
ARG GID=1000
ARG DOCKER_GID=102

# regular group
RUN groupadd \
    --force \
    --gid ${GID} \
    ${NONROOT_USER}

# docker group
 RUN groupadd \
    --force \
    --gid ${DOCKER_GID} \
    docker

# create the passwordless non root user
RUN useradd \
    --uid ${UID} \
    --gid ${GID} \
    --groups ${DOCKER_GID} \
    --create-home \
    --shell /bin/bash \
    ${NONROOT_USER}
RUN passwd -d ${NONROOT_USER}
RUN usermod -aG sudo ${NONROOT_USER}

# chroot script
COPY colima /usr/bin/colima
RUN chmod 755 /usr/bin/colima

# symlinks
RUN ln -s /usr/bin/colima /usr/local/bin/nerdctl
RUN ln -s /usr/bin/colima /usr/local/bin/kubectl

ENV HOME="/home/${NONROOT_USER}"

# backup previous $HOME to get bash utils
RUN mkdir -p /prevhome
RUN chown -R ${UID}:${GID} /prevhome
RUN cp -r ${HOME}/. /prevhome

# reset user back to user
USER ${NONROOT_USER}

WORKDIR /home/${NONROOT_USER}

CMD ["sh", "-c", "(cp -n -r /prevhome/. ${HOME} || echo) && sudo service ssh start && (while true; do sleep 1000; done)"]