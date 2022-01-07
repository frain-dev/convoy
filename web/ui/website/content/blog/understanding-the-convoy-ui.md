---
title: Understanding The Convoy UI
date: 2021-11-11T15:23:31.879Z
description: One of the major issues and problems of webhook over the years has
  been the ability to monitor and understand the state of your webhooks service
  at any time, and that’s a major part of what we’re solving with Convoy as
  opposed to waiting for users to report failures before you know there is a bug
  or something wrong somewhere. Understanding how much of a pain point this is,
  we built a dashboard that helps you solve this problem.
featureImg: /convoy-dashboard_owvlon.png
thumbnail: /convoy-dashboard_owvlon.png
"": []
featurePost: false
author: emmanuel-aina
tag: convoy
---
One of the major issues and problems of webhook over the years has been the ability to monitor and understand the state of your webhooks service at any time, and that’s a major part of what we’re solving with Convoy as opposed to waiting for users to report failures before you know there is a bug or something wrong somewhere. Understanding how much of a pain point this is, we built a dashboard that helps you solve this problem.

![](https://miro.medium.com/max/1400/1*OtJBG3ykPyBsUMtPGJ9T6A.png)

Convoy dashboard helps to monitor 2 important metrics:

* Events sent
* Applications

![](https://miro.medium.com/max/1400/1*nueVgbjiVigwQMx6jHDNoQ.png)

These two things are at your first sight of the dashboard, helping you with the number of events you’ve used convoy to send so far and the number of applications that received those events.

The number of events sent and how they’ve grown over time helps you to have a concept of how your API product has performed so far. You might want to track this daily, weekly, monthly, or yearly. We’ve enabled you to do just that with the chart on the dashboard.

![](https://miro.medium.com/max/1200/1*IXHihGc6Nj7dFequeXqHYg.gif)

Convoy dashboard chart section

## Chart

The chat basically shows you the metric of events sent over time, while making filters (date and frequency) above available for you to easily tweak into your preference.

## Configurations

You can get the config details that your convoy instance is currently running on as it relates to your webhooks activities. We itemized these config details (except the security-protected ones of course) directly beside the chart.

![](https://miro.medium.com/max/1400/1*-23sI6Y7mhhxTO25pTELoA.png)

# Monitoring

Now down to the critical part, monitoring events. The card on the dashboard shows this in full along with the list of apps. The card is tabbed into two different sections.

![](https://miro.medium.com/max/1400/1*znplvJYC5ZkOxpaZFeLYwA.png)

## Events

The default active tab is the events tab that shows all your events, paginated into 20 events per page. The events table highlights basic details you need to see to know the status of each event. You can filter your events by Apps (events sent by a specific app) and date (events sent within a specific date frame).

On clicking each event, you can view the event’s last delivery attempt response and request details i.e the request header details and response body, along with some other details like the IP address, HTTP status, and API version.

![](https://miro.medium.com/max/1200/1*1o7Ipg1s1oLVe9js1Z3Xhg.gif)

## Apps

The next tab to events is the apps tab which has a table of apps receiving events on your system. Listing the apps on the table, the table shows the individual app name, date created, date updated, event no (number of events the app has received), endpoints no (the number of endpoints your system sends endpoints to for that app), and an events button that takes you to the events tab to view events of that specific app.

![](https://miro.medium.com/max/1400/1*d4BS1GSet58HaEyZtncg9Q.png)

Furthermore, in the apps table, you can click on each app to open the details tab and show the individual endpoints of that app.

## Summary

A graph that shows your event activities, your convoy instance configuration details, a filterable table of events that further gives you debuggable details of your events, and a table of apps for an overview of your apps. We’re actively thinking about the best developer experience for users, we’re constantly rethinking the Convoy entire system with backward compatibility and enabling you to focus on the important things.