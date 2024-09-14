#!/bin/bash

# Start D-Bus session daemon
dbus-daemon --session --address=${DBUS_SESSION_BUS_ADDRESS} &

sleep 3

# Start PulseAudio
pulseaudio --start --exit-idle-time=-1

sleep 3

pacmd load-module module-null-sink sink_name=infinitySink