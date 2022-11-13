ARG ARCH="x86_64"

FROM ubuntu-layer-${ARCH}

# use root user for root actions
USER root

# change SSH port to avoid clash
COPY <<EOF /etc/systemd/system/ssh.socket.d/listen.conf
[Socket]
ListenStream=
ListenStream=23
EOF

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
    --no-log-init \
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

# backup previous $HOME to get bash prompt
RUN mkdir -p /prevhome
RUN chown -R ${UID}:${GID} /prevhome
RUN cp -r ${HOME}/. /prevhome

ENV STARTUP_SCRIPT="/usr/bin/start.sh"

# startup script
COPY <<EOF ${STARTUP_SCRIPT}
#!/usr/bin/env sh
set -e

# preserve home dir config
cp -n -r /prevhome/. ${HOME} || echo

# sockets
sudo mkdir -p /var/run
sudo ln -s /host/run/docker.sock /var/run/docker.sock
sudo mkdir -p /var/run/containerd
sudo ln -s /host/run/containerd/containerd.sock /var/run/containerd/containerd.sock
sudo mkdir -p /var/run/buildkit
sudo ln -s /host/run/buildkit/buildkitd.sock /var/run/buildkit/buildkitd.sock
EOF
RUN chmod +x "${STARTUP_SCRIPT}"

# systemd service file
COPY <<EOF /lib/systemd/system/colima-start.service
[Unit]
Description=setup user environment and symlinks

[Service]
User=${NONROOT_USER}
ExecStart=${STARTUP_SCRIPT}

[Install]
WantedBy=multi-user.target
EOF
# enable systemd service
RUN ln -s /lib/systemd/system/colima-start.service /etc/systemd/system/multi-user.target.wants/colima-start.service

# systemd in container recommendations
ENV container=containerd
STOPSIGNAL SIGRTMIN+3

# set default workdir
WORKDIR ${HOME}

# start with systemd
ENTRYPOINT ["/usr/lib/systemd/systemd"]
