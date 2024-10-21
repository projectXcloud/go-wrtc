# syntax=docker/dockerfile:1

# Use the official Go image as the base
ARG VARIANT="1.20"
FROM mcr.microsoft.com/vscode/devcontainers/go:0-${VARIANT}

# Install necessary packages including HAProxy and UFW
RUN apt-get update && \
    apt-get install -y --no-install-recommends \
        pulseaudio \
        mpg123 \
        libpulse-dev \
        pkg-config \
        ca-certificates \
        dbus \
        haproxy \
        ufw \
        gstreamer1.0-tools gstreamer1.0-plugins-base \
        gstreamer1.0-plugins-good gstreamer1.0-plugins-bad \
        gstreamer1.0-plugins-ugly gstreamer1.0-libav && \
    rm -rf /var/lib/apt/lists/*

# Generate machine-id for D-Bus
RUN dbus-uuidgen --ensure

# Configure PulseAudio to run without a real sound card
RUN echo "autospawn = no" >> /etc/pulse/client.conf && \
    echo "daemon-binary = /usr/bin/pulseaudio" >> /etc/pulse/client.conf && \
    echo "enable-shm = false" >> /etc/pulse/client.conf

# Create a non-root user (vscode) if it doesn't exist
ARG USERNAME=vscode
ARG USER_UID=1000
ARG USER_GID=$USER_UID

# Ensure the vscode user and group exist, then add to the group if necessary
RUN id -u $USERNAME &>/dev/null || adduser --uid $USER_UID --disabled-password --gecos "" $USERNAME

# Ensure the group exists before adding the user to it
RUN getent group $USERNAME || groupadd -g $USER_GID $USERNAME
RUN usermod -aG $USERNAME $USERNAME

# Set up permissions for PulseAudio and D-Bus
RUN mkdir -p /run/user/$USER_UID && \
    chown -R $USERNAME:$USERNAME /run/user/$USER_UID

# Backup and remove the default haproxy.cfg
RUN cp /etc/haproxy/haproxy.cfg /etc/haproxy/haproxy.cfg.bak && \
    rm /etc/haproxy/haproxy.cfg

# Copy the custom haproxy.cfg from the host into the container
COPY haproxy.cfg /etc/haproxy/haproxy.cfg

# Set root password to "root"
RUN echo "root:root" | chpasswd

# Switch to the non-root user
USER $USERNAME

# Set environment variables for PulseAudio and D-Bus
ENV XDG_RUNTIME_DIR=/run/user/$USER_UID
ENV DBUS_SESSION_BUS_ADDRESS=unix:path=${XDG_RUNTIME_DIR}/bus

# Ensure the environment variables are available in all shells
RUN echo "export XDG_RUNTIME_DIR=${XDG_RUNTIME_DIR}" >> /home/${USERNAME}/.bashrc && \
    echo "export DBUS_SESSION_BUS_ADDRESS=${DBUS_SESSION_BUS_ADDRESS}" >> /home/${USERNAME}/.bashrc

# Expose the necessary port
EXPOSE 3000

# Set the working directory
WORKDIR /workspaces
