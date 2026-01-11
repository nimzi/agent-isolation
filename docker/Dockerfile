ARG BASE_IMAGE
FROM ${BASE_IMAGE}

WORKDIR /work

COPY --chmod=0755 setup-git-ssh.sh /docker/setup-git-ssh.sh
COPY --chmod=0755 bootstrap-tools.sh /docker/bootstrap-tools.sh
COPY bootstrap-tools.py /docker/bootstrap-tools.py

# Note: cursor-agent CLI is installed by `ai-shell up` after container creation
# This is because /root is mounted as a volume, which would overwrite the installation
# Installing it after container creation ensures it persists in the /root volume

# Keep container running without assuming tail/coreutils.
CMD ["sh", "-c", "while true; do sleep 3600; done"]
