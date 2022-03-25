---
title: App Portal
description: 'Convoy App Portal'
id: app-portal
order: 5
---

# App Portal

We extended the visibility we provide you on the Convoy dashboard to your users through app portal, so that your users can view, debug and inspect events sent to them. While the APIs behind app portal are available to build and customize for yourself, we built app portal so you don't have to go through that stress.

![convoy app portal](../../docs-assets/app-portal-ui.png)

App portal to be usable in three different ways:

1. **As a web component**: enabling you to install it into your existing customer application (that's ease). App portal is currently available for the three of the most popular Angular, React and Vue.
2. **Through a link**: you can just open in a new tab and share with a customer quickly. Note: the token expires, i.e the link will be usable for a limited period of time.
3. **Through an iframe**: you can embed into a vanilla HTML/Javascript application, copy the iframe code from the dashboard and past in to code.

![convoy dashboard app details](../../docs-assets/convoy-dashboard-app-details.png)

## App Portal Iframe

As explained above, the iframe code snippet was made available as the easiest to present app portal to your customers. The token embedded into the iframe code expires, so you can use this [API](https://convoy.readme.io/reference/post_security-applications-appid-keys) to generate a new token whenever your user enters the page with the iframe. Simply replace the `key` gotten from the API response with `{token}` in the example below.

```html[iframe snippet]
<iframe style="width: 100%; height: 100vh; border: none;" src="http://localhost:4200/ui/app-portal/{ token }&appId=291e98cb-4e93-408f-bb5b-d422ff13d12c"></iframe>
```

## App Portal Web Components

### Angular

You can get started with using App Portal in your Angular application by following the three simple steps below:

1. Run `npm i convoy-app` in your existing Angular application to install the package
2. Import `ConvoyAppModule` into your application module as shown below
3. Add `ConvoyApp` to your HTML page

```javascript[app.module.ts]
import { ConvoyAppModule } from 'convoy-app';


@NgModule({
    ...
    imports: [..., ConvoyAppModule],
    ...

    )}

...
```

```html[app.component.html]
...

<convoy-app [token]="token" [apiURL]="apiURL"></convoy-app>

...
```

### React

Adding App Portal to your React application can be done in two steps:

1. Run `npm i convoy-app-react` in your existing React application to install the package
2. Add `ConvoyApp` to your desired page

```javascript[app.js]
import { ConvoyApp } from 'convoy-app-react';
import 'convoy-app-react/dist/index.css';

...

<ConvoyApp token={'token'} apiURL={'apiURL'} />

...
```

### Vue

Adding App Portal to your Vue application can be done in two steps:

1. Run `npm i convoy-app-vue` in your existing React application to install the package
2. Add `ConvoyApp` to your desired page

```javascript[page.vue]
...

<convoy-app :token="token" :apiURL="apiURL" />

...
```
