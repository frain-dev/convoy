---
title: App Portal
description: 'Convoy App Portal'
id: app-portal
order: 5
---

# App Portal

We extended the visibility we provide you on the Convoy dashboard to your users through app portal, so that your users can view, debug and inspect events sent to them. While the APIs behind app portal are available to build and customize for yourself, we built app portal so you don't have to go through that stress.

![convoy app portal](../../docs-assets/app-portal-ui.png)

We built the App portal to be usable in three different ways:

1. **As a web component**: enabling you to install it into your existing customer application (that's ease). App portal is currently available for the three of the most popular Angular, React and Vue.
2. **Through a link**: you can just open in a new tab and share with a customer quickly. Note: the token expires, i.e the link will be usable for a limited period of time.
3. **Through an iframe**: you can embed into a vanilla HTML/Javascript application, copy the iframe code from the dashboard and past in to code.

**Note**: The token embedded into the iframe code also expires, so you can use this [API](https://convoy.readme.io/reference/post_security-applications-appid-keys) to generate a new token whenever your user enters the page with the iframe.

## Iframe Link Structure

```json[Sample Config]
{
  "database": {
    "dsn": "mongodb://root:rootpassword@localhost:27037"
  },
  "queue": {
    "type": "redis",
    "redis": {
      "dsn": "redis://localhost:8379"
    }
  },
  "server": {
    "http": {
      "port": 5005
    }
  },
  "auth": {
    "type": "none"
  },
  "strategy": {
    "type": "default",
    "default": {
      "intervalSeconds": 125,
      "retryLimit": 15
    }
  },
  "ui": {
      "type": "none"
  }
}
```
