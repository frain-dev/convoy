## 0.8.2

-   [Feature] Set Advanced Signatures by default for Incoming projects.
-   [Enhancement] Refactored Convoy CLI Implementation. #1290
-   [Enhancement] Improved Project Onboarding #1335
-   [Enhancement] Improved Delivery Attempt Details page #1341
-   [Enhancement] Update Subscriptions Card for Incoming Projects. #1349
-   [Enhancement] Update portal links page #1348
-   [Enhancement] Update portal links page #1348
-   [Enhancement] Improve source, subscription and endpoint forms #1335
-   [Bugfix] Event Log pagination bug #1346
-   [Bugfix] Date picker bug fix #1355

## 0.8.1

-   [Enhancement] Improved stats query #1256
-   [Enhancement] Add analytics and storage policy settings in convoy.josn #1284 #1324
-   [Feature] Set notifications by default #1251
-   [Feature] Add raw invite link in invite email #1279
-   [Feature] Allow user re-generate api key for projects #1277

## 0.8.0

-   [Feature] Filter by HTTP Headers #1240 #1232 #1249
-   [Feature] Add force retry and batch retry to event logs #1237
-   [Feature] Add convoy version to private pages #1230
-   [Feature] Add frontend analytics and welcome email #1209
-   [Feature] Build source-maps to separate folder #1216
-   [Feature] Change group to project #1207
-   [Feature] Filter events using event types and subscriptions #1205 #1217
-   [Feature] Add custom domain support #1203
-   [Feature] use central logger everywhere #1176
-   [Feature] Deprecate Apps in favor of Endpoints #1169 #1159 #1069 #1158
-   [Feature] Deprecate `document_status` in favor of `delete_at` #1123 #1168
-   [Feature] Filter events by the subcription filtering #1096 #1116
-   [Feature] Added support for http connect proxy #1146
-   [Enhancement] Deprecate subscription status field #1243 #1234
-   [Enhancement] Update email verification flow #1227
-   [Enhancement] Improved onboarding forms #1245 #1244 #1246
-   [Enhancement] MaxIngestSize can be configured per group/project #1160
-   [Enhancement] Use mongo aggregations when quering multuple collections #1611 #1153
-   [Enhancement] Redirect user to previous page on login after session timeout #1154
-   [Enhancement] Add response body to endpoint disabled notification email/slack message #1141 #1152
-   [Enhancement] Email verification check after signup #1208 #1186
-   [Enhancement] Add support for building alpine images #1108
-   [Enhancement] Add api key migrations #1166
-   [Bugfix] Add raw value to event payload #1236
-   [Bugfix] Persist advanced signatures config #1233
-   [Bugfix] Fix migration tool #1226
-   [Bugfix] Token modal on project creation now show visual feedback #1242
-   [Bugfix] Show event deliveries in correct order #1202 #1157
-   [Bugfix] Used correct type for rate limit duation #1201
-   [Bugfix] Fixed events in discarded state getting stuck #1200
-   [Bugfix] Fixed events should be created regardless of subscriptions logic
-   [Bugfix] Enable default retention policy duration #1163
-   [Bugfix] fixed hobby deployment installation script #1139 #1136 #1135 #1134
-   [Bugfix] Update navbar on creating project on new organisation #1250
-   [Bugfix] Update validation check for create project form #1248

## 0.7.6

-   [Bugfix] Preserve whitespaces in event payload #1223

## 0.7.5

-   [Enhancement] Filter event deliveries by subscriptions #1192
-   [Bugfix] fix prism import error #1194
-   [Bugfix] correctly use query string in app portal #1195

## 0.7.4

-   [Enhancement] hide cli keys and devices on app portal #1184
-   [Bugfix] pass query params along in shim #1185

## 0.7.3

-   [Enhancement] Set app portal key expiration to 30 days #1170 #1171

## 0.7.2

-   [Bugfix] Fix Max response size CLI override #1098
-   [Feature] Advanced Signatures Core Implementation #1053 #1100 #1103 #1104
-   [Feature] Refactored Logging Implementation #1051
-   [Bugfix] Update endpoint with custom authentication #1119 #1106 #1107 #1105
-   [Enhancement] Add support for building alpine image #1108

## 0.7.1

-   [Bugfix] Fix Max response size CLI override #1098

## 0.7.0

-   [Feature] Add support for feature flags #1054
-   [Enhancement] Enable source filter on Events dashboard #1058 #1043
-   [Bugfix] Use configured max request size in ingest handler #1086
-   [Feature] Use mongo aggregation pipeline to fetch subscriptions #1073
-   [Feature] Run Convoy in Headless mode #1034
-   [Enhancement] New UI Onboarding #1048
-   [Bugfix] Fixed retention policies task handler #1049
-   [Bugfix] Set default body for ingested events to empty JSON #1062
-   [Feature] Add support for endpoint authentication #1045
-   [Bugfix] The change stream should not be killed when an error occurs in the handler #1061
-   [Enhancement] Fix build warnings #1089
-   [Bugfix] Several UI fixes #1087

## 0.7.0-rc.2

-   [Feature] Create cli api keys and manage devices from the app portal #983
-   [Bugfix] Fix Subscription Config Inheritance For Event Deliveries #995
-   [Enhancement] Implement cascading delete for groups, apps and sources #1037
-   [Enhancement] Add help to project sidebar #1032

## 0.7.0-rc.1

-   [Feature] Add a command to run convoy migrations #989 #996 #993
-   [Feature] Storybook setup #927
-   [Feature] Add load test scripts #997
-   [Feature] Add convoy websocket cli event streaming server and client #931
-   [Feature] Add support for custom headers for outgoing events #1012
-   [Bugfix] Use typesense multi search #994
-   [Bugfix] Fix retention policy export #998
-   [Bugfix] Allow discarded events to be retried #1016
-   [Bugfix] Validate source providers separately #1024
-   [Enhancement] Fixed API spec annotations #1005
-   [Enhancement] Refactor Store Dependency #1006

## 0.6.8

-   [Bugfix] Fix subscriptions page error #1026
-   [Bugfix] Add deleted_at to MongoDB indexes #1027
-   [Bugfix] Dismiss modal when a source is deleted #1026

## 0.6.7

-   [Bugfix] Fix analytics query #1001
-   [Bugfix] Fix middleware layer for app portal #988
-   [Bugfix] Fix endpoint notification email bug #981

## 0.6.6

-   [Bugfix] Fix project statistics lookup query #978 #979

## v0.6.5

-   [Feature] Add support for user registration. #959
-   [Bugfix] Updated event dashboard with application and source metadata. #960
-   [Bugfix] Allow re-invitation of a previously cancelled invitation. #967
-   [Enhancement] Projects scoped indexing and search. #945
-   [Enhancement] Improved notification system. #954

## v0.6.4

-   [Bugfix] Fixed a bug where event deliveries could not be force retried #938
-   [Bugfix] Changed the group/project rate limit duration type from string to int #938
-   [Bugfix] Fixed a bug where team invites could not be canceled on the UI #940
-   [Enhancement] Add an environment variable for the typesense collection name #939

## v0.6.3

-   [Enhancement] change app portal url from `/app-portal` to `/app` #924
-   [Enhancement] App portal improvements #918

## v0.6.2

-   [Feature] Added Proxy Events without Verification #906
-   [Enhancement] Reliably forward Incoming events request headers #895
-   [Bugfix] Fixed force retry bug #891 #890

## v0.6.1

-   [Bugfix] Fixed API response for force retry endpoint #892, #897
-   [Bugfix] Changed create configuration to use a post request #896

## v0.6.0

-   [Feature] Add Support for Custom Sources (Twitter, Shopify) #869, #833, #826
-   [Feature] Add Support for Retention Policies #839, #879
-   [Enhancement] Updated UI Architecture to use Tailwind CSS #816
-   [Enhancement] Optimised UI to reduce initial bundle size #879
-   [Enhancement] Allow all workers to run in a single cluster #876
-   [Enhancement] Add cancelled status on org invite #812
-   [Bugfix] Fixed wrong FindMany query in subscriptions.go #858
-   [Bugfix] Enabled JWT configuration with env variables #813

## v0.6.0-rc.4

-   [Bugfix] Fixed bug in correctly using datastore FindMany method #856

## v0.6.0-rc.3

-   [Change] Changed host to instance id in the analytics #821
-   [Enhancement] Add support for JWT environment variables #813
-   [Bugfix] Fix analytics query #825
-   [Bugfix] UI bug fixes #814

## v0.6.0-rc.2

-   [Bugfix] Dereference slice when finding source subscriptions #808
-   [Bugfix] Use redis client in scheduler #807
-   [Bugfix] Fixed an issue where the source type would not be updated when updating a Github source
-   [Bugfix] Fixed an issue where the application details would not be loaded when creating a subscription
-   [Bugfix] Fixed an issue where an organization created by a user would not show on the top bar to be selected

## v0.6.0-rc.1

-   [Change] Introduce organisations to partition different sets of projects.
-   [Change] Deprecate file authentication and authorisation. You no longer specify authentication credentials from convoy.json. User and permission details are now persisted to the DB and use jwt for authentication.
-   [Change] All users are now super users in the OSS core.
-   [Change] Sentry error tracking has been deprecated. Only New relic is supported for error tracking.
-   [Change] Revamped UI. The former convoy dashboard was revamped to enable more management of several vital resources - users, projects, applications, endpoints, sources, and subscriptions.
-   [Change] require_auth has been deprecated. All endpoints will now require authentication.
-   [Feature] Add Github Custom source #792 #791
-   [Enhancement] Change base_url config variable to host #754
-   [Enhancement] Set default event types when filter config is nil #783
-   [Enhancement] Switched background job system to asynq. #711
-   [Enhancement] Add toggle subscription status endpoint #784
-   [Enhancement] Autogenerated webhook secrets use alphanumeric secrets #751
-   [Enhancement] Use asynq for the scheduler. #745
-   [Bugfix] Prevent an organisation owner from being deactivated #781
-   [Bugfix] Fix events ingestion to create event flow #744
-   [Bugfix] Fixed a race condition that could occur when making an application endpoint #790
-   [Bugfix] Fixed app portal link. #790
-   [Bugfix] Use correct arguments for API key verifier #779
-   [Bugfix] Fixed switching between organisations #775
-   [Bugfix] Return proper error from SendNotification #764
-   [Bugfix] Fixed filters in events and event deliveries #718
-   [Bugfix] Fixed loaders in projects page #724

## v0.6.0-rc

-   [Enhancement] Optimize group statistics query #677
-   [Enhancement] pause retry count for rate limit errors #676
-   [Enhancement] Add groupID arg to application datastore methods
-   [Feature] Add Typesense search backend #652
-   [Enhancement] Added integration tests #647 #655 #656 #661 #643 #638
-   [Feature] Add support for storing events for disabled apps #663
-   [Enhancement] Integrate disq as a replacement for taskq #667
-   [Enhancement] Fix mongodb index model type #671
-   [Bugfix] Update endpoints secret #640
-   [Bugfix] Prevent duplicate app names #635
-   [Feature] Force retry on App portal #633

## v0.5.3

-   [Feature] Add update scripts for migrating from v0.4 to v0.5 #611
-   [Enhancement] Changed the way events are created #592
-   [Documentation] Add GroupId to swagger documentation #617
-   [Documentation] Fix build command in README.md #600
-   [Documentation] Fix convoy.json.example #603
-   [Enhancement] Add unit tests for the service layer #596 #594 #593 #589
-   [Enhancement] Increase test coverage in server package #584 #581 #565
-   [Enhancement] Add unit and e2e test for dashboard component #580 #612
-   [Enhancement] Updated UI on dashboard and app portal #616 #590
-   [Enhancement] Improve loaders for dashboard and app portal #614 #616
-   [Feature] Add slack notification system #562
-   [Feature] Add Force Resend to App Portal API #579
-   [Enhancement] Configurable Replay attacks on groups #567

## v0.5.0

-   [Feature] Convoy can now be configured with only environment variables and/or cli flags #511 #520
-   [Feature] Add rate limit to api and ui endpoints using the group id #486
-   [Feature] Add configuration option to set rate limits on application endpoints
-   [Feature] Add configuration option to set endpoint timeout duration #550
-   [Feature] Add support for disabling an application #527
-   [Enhancement] Removes the need for always passing the groupID as a query string while authenticating with an API Key. #535
-   [Bugfix] Add the correct event delivery status for matched endpoints #503
-   [Feature] Convoy now supports replay attack prevention by providing a timestamp in the signature header #528 #537
-   [Feature] Convoy now uses filters for batch retrying event deliveries.
-   [Feature] Convoy can now force resend successful event deliveries.
-   [Enhancement] Introduced a service layer into the code architecure #532 #547 #555 #552

## 0.4.10

-   [Feature] We can now download convoy binaries from package managers #459
-   [Enhancement] Add support for embedding convoy version file #454
-   [Feature] Expose taskq queue metrics #476
-   [Feature] Added support for embedding an App portal in a 3rd pary app #463

## 0.4.0

-   [Enhancement] Replace bbolt implementation with badger #417
-   [Enhancement] Add compound indexex for events, eventdeliveries and applications #418

## 0.4.0-rc

-   [Feature] Added in memory queue using taskq #342
-   [Feature] Added in memory database using bolt #390 #388 #347 #348 #343
-   [Feature] Native Realm Implementation #349
-   [Feature] Add Group statistics #330
-   [Feature] Spilt Server Worker CLI #350
-   [Feature] Add support for New Relic #264
-   [Feature] Add application search #336
-   [Bugfix] Configure Max response size to store #345
-   [Enhancement] Fix slow dashboard summary endpoint #381
-   [Enhancement] Improve Request Validation #365
-   [Enhancement] Event deliveries search apps filter #382

## 0.3.0

-   [Bugfix] Fixed hash computation bug #269
-   [Bugfix] Bundled UI into binary builds. #273
-   [Bugfix] Don't enqueue discarded events #253
-   [Enhancement] Build Convoy dashboard UI into npm package in `dashboard-component` #272

## 0.3.0-rc

-   [Feature] URL/Events: User can specify events to each endpoint.
-   [Feature] Batch Retries: User can batch retry multiple event deliveries at once.
-   [Feature] Unified auth configuration for both UI and API.
-   [Feature] Added minimal rbac for super user, admin and ui admin.
-   [Feature] New tab to view event deliveries for events.
-   [Feature] Filter event deliveries by delivery status, app and date range
-   [Feature] View event deliveries status for each event from event's details section.
-   [Feature] Introduced Groups: To support multi-tenancy for multiple products to pipe events as separete groups.
-   [Feature] Persist events and event deliveries filters, active group and active logs tab with page reload.

## 0.2.0

-   [Feature] Add disable events and send email notifications.
-   [Feature] Re-activate endpoints by re-trying a non-successful event.
-   [Feature] Enable SMTP configuration.
-   [Enhancement] Improved Delivery Attempt Page.
-   [Enhancement] Event log filtering by Applications and Date.
-   [Enhancement] Changed organisations to groups throughout app.
-   [Enhancement] Changed /apps to /applications
-   [Enhancement] Create default group on app startup.
-   [Enhancement] Clicking events button from apps table now automatically filters events by clicked app.
-   [Enhancement] Convoy config details now shows on dashboard.
-   [Enhancement] Created at and Next retry on events table now shows time instead of date.
-   [Enhancement] Improved table pagination.
-   [Enhancement] Events table now grouped by date.
-   [Enhancement] Manually retried events now identifiable by a retry icon on events table.
-   [Enhancement] Event status now differentiated by status color.
