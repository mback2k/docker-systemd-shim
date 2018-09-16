docker-systemd-shim
===================
This Go program is a compatibility shim to allow the easy control of docker
containers via systemd service units.

[![Build Status](https://travis-ci.org/mback2k/docker-systemd-shim.svg?branch=master)](https://travis-ci.org/mback2k/docker-systemd-shim)

Usage
-----
The following is an example of a systemd.service unit file which uses this shim to control a container:

```
[Unit]
Description=Docker container: your-container-name
Wants=network.target
After=docker.service
Requires=docker.service

[Service]
Type=notify
ExecStart=/usr/local/sbin/docker-systemd-shim --container=your-container-name
Environment=DOCKER_API_VERSION=1.38

[Install]
WantedBy=docker.service
```

Replace `your-container-name` with the name of the container you want to control with this unit file.
