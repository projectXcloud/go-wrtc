# syntax=docker/dockerfile:1

# Use the official Go image as the base
ARG VARIANT="1.20"
FROM mcr.microsoft.com/vscode/devcontainers/go:0-${VARIANT}

# Install necessary packages
RUN apt-get update && \
    apt-get install -y --no-install-recommends \
        pulseaudio \
        mpg123 \
        ffmpeg \
        libpulse-dev \
        pkg-config \
        ca-certificates \
        dbus \
        && \
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

# Ensure the vscode user exists
RUN id -u $USERNAME &>/dev/null || adduser --uid $USER_UID --disabled-password --gecos "" $USERNAME

# Set up permissions for PulseAudio and D-Bus
RUN mkdir -p /run/user/$USER_UID && \
    chown -R $USERNAME:$USERNAME /run/user/$USER_UID

# Switch to the non-root user
USER $USERNAME

# Set environment variables for PulseAudio and D-Bus
ENV XDG_RUNTIME_DIR=/run/user/$USER_UID
ENV DBUS_SESSION_BUS_ADDRESS=unix:path=${XDG_RUNTIME_DIR}/bus

# Ensure the environment variables are available in all shells
RUN echo "export XDG_RUNTIME_DIR=${XDG_RUNTIME_DIR}" >> /home/${USERNAME}/.bashrc && \
    echo "export DBUS_SESSION_BUS_ADDRESS=${DBUS_SESSION_BUS_ADDRESS}" >> /home/${USERNAME}/.bashrc

# Expose the necessary ports
EXPOSE 8000 50000 6080

# Set the working directory
WORKDIR /workspace

# # Optional: Install ufw to control ports
# RUN apt-get update && apt-get install -y ufw && \
#     ufw default deny incoming && \
#     ufw default allow outgoing && \
#     ufw allow 8000 && \
#     ufw allow 50000 && \
#     ufw allow 6080 && \
#     ufw --force enable

