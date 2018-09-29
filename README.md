docker-systemd-shim
===================
This Go program is a compatibility shim to allow the easy control of docker
containers via systemd service units.

[![Build Status](https://travis-ci.org/mback2k/docker-systemd-shim.svg?branch=master)](https://travis-ci.org/mback2k/docker-systemd-shim)
[![GoDoc](https://godoc.org/github.com/mback2k/docker-systemd-shim?status.svg)](https://godoc.org/github.com/mback2k/docker-systemd-shim)

Usage
-----
The following is an example of a systemd.service unit file which uses this shim to control a container:

```
[Unit]
Description=Docker container: your-container-name
Wants=network.target
After=docker.service
Requires=docker.service
# You could also specify dependencies on other units, including those managed with this shim
#After=your-other-container.service
#Requires=your-other-container.service

[Service]
Type=notify
ExecStart=/usr/local/sbin/docker-systemd-shim -container your-container-name
Environment=DOCKER_API_VERSION=1.38
Restart=on-failure
PrivateTmp=true
ProtectHome=true
ProtectSystem=full

[Install]
# You could either make this service start, stop and restart together with docker
WantedBy=docker.service
# or make it just start and stop with the system, but not restart with docker
#WantedBy=multi-user.target
```

Replace `your-container-name` with the name of the container you want to control with this unit file.
