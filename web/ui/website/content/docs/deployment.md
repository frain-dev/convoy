---
title: Deployment
description: 'Techniques to deploy convoy to the various cloud platforms'
id: deployment
order: 3
---

# Deploying Convoy

Convoy is distributed as binaries and docker images, and can be deployed to virtually any platform containers can be deployed to.

### Deploying to Heroku

This section contains information on how to deploy Convoy to Heroku.

#### Prequisites

1. Heroku CLI
2. Heroku Application, configured with environment variables.

---

Then you can follow these steps:

1. ### Pull Docker Image

```bash
$ docker pull ghcr.io/frain-dev/convoy:v0.2.4
```

2. ### Login to Heroku Container Registry

```bash
$ heroku container:login
```

3. ### Push Convoy Image

```bash
$ docker tag ghcr.io/frain-dev/convoy:v0.2.4 registry.heroku.com/<heroku-app-name>/<process-type>
$ docker push registry.heroku.com/<heroku-app-name>/<process-type>
```

4. ### Create a new release

```bash
$ heroku container:release web --app <heroku-app-name>
```
