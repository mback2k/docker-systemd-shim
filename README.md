docker-systemd-shim
===================
This Go program is a compatibility shim to allow the easy control of docker
containers via systemd service units.

[![Build Status](https://travis-ci.org/mback2k/docker-systemd-shim.svg?branch=master)](https://travis-ci.org/mback2k/docker-systemd-shim)
[![GoDoc](https://godoc.org/github.com/mback2k/docker-systemd-shim?status.svg)](https://godoc.org/github.com/mback2k/docker-systemd-shim)

Installation
------------
You basically have two options to install this Go program package:

1. If you have Go installed and configured on your PATH, just do the following go get inside your GOPATH to get the latest version:

```
go get -u github.com/mback2k/docker-systemd-shim
```

2. If you do not have Go installed and just want to use a released binary,
then you can just go ahead and download a pre-compiled Linux amd64 binary from the [Github releases](https://github.com/mback2k/docker-systemd-shim/releases).

Finally put the docker-systemd-shim binary onto your PATH and make sure it is executable.

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

Disclaimer
----------
This tool is meant as a supplement to standalone or unmanaged Docker containers and
not as a replacement for Kubernetes, Docker Swarm or the docker run CLI command.
This tool can only control previously created and already existing docker containers.

I personally use this tool to manage pre-requisite containers for my Docker Swarm
cluster, for example ndppd and tinc running inside containers managed via Ansible.

License
-------
Copyright (C) 2018  Marc Hoersken <info@marc-hoersken.de>

This software is licensed as described in the file LICENSE, which
you should have received as part of this software distribution.

All trademarks are the property of their respective owners.
