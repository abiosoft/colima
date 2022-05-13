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

# home directory should match Lima's
ENV HOME="/home/${NONROOT_USER}.linux"

# create the passwordless non root user
RUN useradd \
    --uid ${UID} \
    --gid ${GID} \
    --groups ${DOCKER_GID} \
    --create-home \
    --home-dir ${HOME} \
    --shell /bin/bash \
    ${NONROOT_USER}
RUN passwd -d ${NONROOT_USER}
RUN usermod -aG sudo ${NONROOT_USER}

# chroot script
COPY colima /usr/bin/colima
RUN chmod 755 /usr/bin/colima

# symlinks
RUN ln -s /usr/bin/colima /usr/local/bin/docker
RUN ln -s /usr/bin/colima /usr/local/bin/nerdctl
RUN ln -s /usr/bin/colima /usr/local/bin/kubectl

# sockets
RUN mkdir -p /var/run
RUN ln -s /host/run/docker.sock /var/run/docker.sock
RUN mkdir -p /var/run/containerd
RUN ln -s /host/run/containerd/containerd.sock /var/run/containerd/containerd.sock
RUN mkdir -p /var/run/buildkit
RUN ln -s /host/run/buildkit/buildkitd.sock /var/run/buildkit/buildkitd.sock

# backup previous $HOME to get bash prompt
RUN mkdir -p /prevhome
RUN chown -R ${UID}:${GID} /prevhome
RUN cp -r ${HOME}/. /prevhome

# startup script
COPY <<EOF /usr/bin/start.sh
#!/usr/bin/env/sh
set -e
cp -n -r /prevhome/. ${HOME} || echo
sudo service ssh start
while true; do sleep 1000; done
EOF

# revert user to rootless user
USER ${NONROOT_USER}

WORKDIR ${HOME}

# use recommended init process
ENTRYPOINT ["tini", "--"]
CMD ["sh", "/usr/bin/start.sh"]