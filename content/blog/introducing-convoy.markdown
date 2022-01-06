---
title: Introducing Convoy
date: 2021-10-25T17:10:13.696Z
description: After weeks of work, I’m pleased to announce our new tool — built
  to send webhooks efficiently. While building out our initial API on
  third-party monitoring, every user we spoke to wanted asynchronous events —
  “Do you provide webhooks to notify us about downtime?”.
featureImg: https://res.cloudinary.com/frain/image/upload/c_crop,f_auto,q_auto,w_367,x_41,y_41/v1641490017/blog/Introducing%20Convoy/convoy-dashboard_okeuo8.png
thumbnail: https://res.cloudinary.com/frain/image/upload/c_fill,h_179,w_461,x_0,y_0/v1641490017/blog/Introducing%20Convoy/convoy-dashboard_okeuo8.png
tags:
  - convoy
  - webhooks
author: subomi-oluwalana
---
# Introducing Convoy

After weeks of work, I’m pleased to announce our new tool — built to send webhooks efficiently. While building out our initial API on third-party monitoring, every user we spoke to wanted asynchronous events — “Do you provide webhooks to notify us about downtime?”. We looked around, and sadly, we couldn’t find a great tool — language agnostic and cloud-native to build this. So we built it.

![](https://miro.medium.com/max/1400/1*LlrJI0W8XxNNrha1cpe_mg.png)

Convoy Dashboard

# Problems & Our Solutions

On the surface, when you think of webhooks it is simply HTTP Push. If you think this, you might be partly correct, but the story doesn’t end there. Let’s explore:

## Reliability

Essentially, a failed webhooks event has a direct customer impact. A failed event from Paystack means a customer won’t receive value for their purchase on Domino’s while the customer has received a debit alert. It means a Piggyvest customer will not see their top-up while the customer has received a debit alert. A failed webhooks event from Termii means you can’t show your beautiful UX of successful OTP delivery. A failed webhooks event from Mono means you cannot notify your customers of a successful account integration even when you’ve received an authentication token. For the non-technical, webhooks are the glue that ties modern apps together to create endless possibilities. We choose to build Convoy in Golang, and distribute binaries & docker images. Go is more or less the “de-facto language*”* for building highly available and reliable services in the Cloud.

![Golang and Docker](https://miro.medium.com/max/1400/1*xR4T978ZKbQUDORnx0w1KQ.jpeg)

Golang and Docker

## Developer Experience

Ok, let’s be honest. In the cloud, everything fails — I mean literally \[Looking at Facebook :( ]. The question is; what is our mean time to recovery (MTTR). How fast can we resend failed events? Do we have to reach out to Paystack to resend events that didn’t make it? Or events that were sent but we didn’t handle properly? Oh. Flutterwave is sending the wrong data format? How do we verify this hypothesis fast? Can we see what was sent & what our server’s response body is? Who’s the culprit — DNS? Nginx? Essentially, the developer experience around your webhooks infrastructure becomes critical to debugging and recovery. We built Convoy with a web interface, that should enable both Paystack & Paystack’s customers to filter through event logs and resend events easily and fast.

![](https://miro.medium.com/max/1400/1*mTpTVnnR_EXUSrfimzOXFw.png)

Convoy dashboard with retry button enabled

## Monitoring and Alerts

Alright, we get it. I can search my event logs and debug fast. What’s left in webhooks. You see, a successful event means your servers respond with a 200. If your server consistently fails to return a 200 for whatever reason, there’s no reason to continue bombarding the endpoint with more events; It’s a dead endpoint. But how do you know & triage quickly? Essentially, you can implement different solutions — uptime monitoring, monitor average request/minute on your webhooks route, and flag it when you’re below a certain threshold. But obviously, the webhooks provider can see the failed delivery attempts over x time or x events. Without a monitoring and alerts solution, your customers become your Prometheus ( ._.). With Convoy after an endpoint consistently fails we disable the endpoint and send an email to the developers to triage.

![](https://miro.medium.com/max/1400/1*8as-x-tv8n8Kh677FgEJpQ.png)

## Other Problems

> It is common to believe we need Stripe quality webhooks. But I disagree, what we need is Quality webhooks for everyone. Stripe’s webhooks is optimised largely for Security and Developer Experience. Twilio is optimised for performance. PagerDuty is optimised for flexibility. Convoy democratises all these complexities in a single binary.

Honestly, I can go on and on, because I’m so excited about this release. You see, there exist a myriad of other problems around a proper webhook delivery infrastructure. It is common to believe we need Stripe quality webhooks. But I disagree, what we need is Quality webhooks for everyone. Stripe’s webhooks are optimized largely for Security and Developer Experience. Twilio is optimized for performance. PagerDuty is optimised for flexibility. Convoy democratises all these complexities in a single binary.

# The Future

Essentially, just like Redis is to key-value storage, and Gitlab is to DevOps, Convoy is to webhooks. We think it’s possible, and we’re yet to scratch the surface of the experience we want to achieve/is needed, but we think we’re off to a good start and we’re willing to share with the community. This future includes but is not limited to — Rate limiting, Static IP, High availability, Headless (i.e. run Convoy without a third-party queue & storage). Convoy has been running in production in [Buycoins](https://buycoins.africa/) for the past month, and a few folks are deploying e.g. [Termii](https://termii.com/) & [GetWallets](https://www.getwallets.co/).

Finally, we built Convoy as an [open-source](https://github.com/frain-dev/convoy) project, distributed as Go binaries and a docker image. If you’d like to join the waitlist for Convoy Cloud, please head over to our [product site](https://getconvoy.io/) and drop your email.

Cheers