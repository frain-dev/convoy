---
title: Deployment
description: 'Techniques to deploy convoy to the various cloud platforms'
id: deployment
order: 4
---

# Deploying Convoy

Convoy is distributed as binaries and docker images, which means you can run it in any VM environment or to any platform containers can be deployed to.

### Deploying to Heroku

This section contains information on how to deploy Convoy to Heroku.

#### Prequisites

1. Heroku CLI
2. Heroku Application, configured with environment variables.

---

Then you can follow these steps:

1. ### Pull Docker Image

```bash
$ docker pull ghcr.io/frain-dev/convoy:v0.5.2
```

2. Login to your Heroku account

```bash
$ heroku login
``` 

3. ### Login to Heroku Container Registry

```bash
$ heroku container:login
```
if this doesn't work you can use the docker cli to auth on heroku's registry

```bash
$ heroku auth:token | sudo docker login --username=_ \
		--password-stdin registry.heroku.com
```

4. ### Push Convoy Image

```bash
$ docker tag ghcr.io/frain-dev/convoy:v0.5.2 registry.heroku.com/<heroku-app-name>/<process-type>
$ docker push registry.heroku.com/<heroku-app-name>/<process-type>
```

5. ### Create a new release

```bash
$ heroku container:release web --app <heroku-app-name>
```

# Deploying to Digital Ocean (or a VM)

This section contains information on how to run and deploy Convoy to a Digital Ocean VM as a systemd service

#### Prequisites

1. A VM running Ubuntu 20.04 or higher
2. Suggested specs: 4GB RAM, 2vCPUs

---

1. Create the service file.

```bash
$ vim /lib/systemd/system/convoy.service
```

2. Configure the service

```bash 
[Unit]
Description=Convoy is a fast & secure webhooks service.
After=network.target

[Service]
Type=simple
ExecStart=/usr/bin/convoy server --config=/etc/convoy/convoy.json

[Install]
WantedBy=multi-user.target
```
3. Enable the service

```bash
$ systemctl enable convoy.service 
```

4. Start the service

```bash
$ systemctl start convoy.service
```

4. Reload the deamon

```bash
$ systemctl daemon-reload
```

5. Create a folder in /etc
```bash
$ mkdir /etc/convoy
```

6. Move your config file the folder
```bash
$ mv convoy.json /etc/convoy/convoy.json
```

7. Reload the service
```bash
$ systemctl resatrt convoy.service
```