# 25.2.1

### Features

-   Re-enable Meta-Events (#2244)
-   Add manual instrumentation (#2240)
-   Copy migration files to container (#2238)

### Enhancements

-   Improve Tracing: Add Event ID To Workflow Stages (#2245)
-   Updated credential encryption (#2233)
-   Updated Redis cache encryption TTL (#2231)
-   Updated read-replicas to rely on feature flags (#2226)

### Bug Fixes

-   Fix panic in backup project data handler (#2232)
-   Fix: add image SHA to Dockerfile (#2234)
-   Fix: fixed hash length for rolling updates image tag (#2225)
-   Fix: fixed publishing manifest file (#2224)
-   Fix: fixed bug building rolling updates manifest (#2223)

### Chores

-   Chore(deps): Bump posthog-js from 1.36.0 to 1.127.0 in /web/ui/dashboard (#2228)
-   Chore(deps): bump github.com/docker/docker (#2229)

# 25.1.1

### Features

-   Added database read replica support #2195
-   Create event types whenever a subscription is created or updated #2201
-   Localhost endpoints can now be configured #2216
-   Added credential encryption lifecycle management #2202 #2204 #2215 #2213
-   Added Retention Policies with Partitioning #2194 #2198

### Enhancements

-   Add workflow to build rolling images #2217 #2218
-   Remove cache implementation #2206
-   added host to init sso flow #2219

### Bug Fixes

-   Remove multiple subscription for an endpoint setting #2197
-   Fixed a bug where some gzipped bytes couldn't be inserted into the db column #2214 #2203
-   Fixed duplicate metrics collection #2220 #2221
-   Remove acknowledged_at from the event search query #2205
-   Fixed a bug where the subscription filter form didn't persist deleted configurations #2210 #2209
-   Fixed portal link UI issues with subscriptions, endpoints, and license check #2208
-   Fixed portal link creation by owner id #2207

# 24.11.1

### Features

-   Added endpoint circuit breaking. #2120
-   Added rate limiting to server apis. #2166
-   Added allow and block IP Lists #2169
-   Added event types #2180
-   Added Enterprise SSO #2175

### Enhancements

-   Removed retry configuration from subscriptions. #2161
-   Redesigned event processing system to use a workflow approach. #2131
-   Dashboard UI Improvements #2167
-   Added missing index for fetching delivery attempts #2172
-   Added version header to responses #2174
-   Optimized queries for prometheus metrics #2179

### Bug Fixes

-   Fix flatten multiple top level operator bug #2168
-   Delete an invitation after decline or accept #2171

# 24.9.2

### Enhancements

-   Set CORS allow origin to the configured convoy instance host #2152
-   Properly format token expiry time forgot/reset password email #2152
-   Restrict loading of the Convoy dashboard (except portal link dashboards) in an iframe #2153

# 24.9.1

### License Changes

-   Moved ee rbac into community and renamed to multiplayer mode #2146
-   Setting the worker/agent execution mode can only be done using a license #2155

### Features

-   Added support for inbound webhooks that use urlencoded content type #2119

### Enhancements

-   HTTP ingest now uses the instance ingest rate #2156
-   Show endpoint authentication config in the UI #2141

### Bug Fixes

-   Make forgot password response obscure #2144

# 24.8.2

### Bug Fixes

-   fixed a retention policy bug where only empty files were uploading to s3

# 24.8.1

### Features

-   Added end to end latency metrics #2079
-   Add support for inbound webhooks that use form data #1998
-   Added e2e test suite using test containers #2083
-   Add license feature gating #2114 #2132 #2134
-   Change License to Elastic License v2.0 #2124

### Enhancements

-   Move retention policy to instance config #2011
-   Update event data openapi types #2088
-   Refactor agent, worker and ingest entry points #2082
-   Refactored Exponential Backoff Implementation #2073
-   Remove instance configuration page #2085
-   Set default signature value to advanced from UI #2090
-   Add fanout for pubsub ingest #2099
-   Events ingested in incoming projects would now respond with a 413 for oversized payloads #2095
-   The agent component can now bootstrap a fresh instance #2111
-   Don't return an error when an owner id has no registered endpoints #2112
-   Split delivery attempts from event deliveries #2092
-   Add auth to metrics and queue monitoring routes #2115
-   Updated integration test suite #2100
-   Refactored feature flags implementation #2105
-   Push docker images to DockerHub #2122
-   Add owner id to event delivery response #2129

### Bug Fixes

-   Fixed a bug in positional array filter #2086
-   Fix count & batch retry queries #2089
-   Fixed a bug where api responses from v2024-04-01 to v2024-01-01 were not properly migrated #2087
-   Update UI Dependencies #2097
-   Fixed a migration bug where default column values were not set #2103
-   Fixed a bug where the wrong delay duration was used when scheduling an event delivery for retry #2110
-   Fixed a bug where other events were retried from a portal link because the endpoint filter wasn't applied #2116

# 24.6.4

-   fixed a bug where the pubsub ingester won't start when there aren't any projects

# 24.6.3

### Bug Fixes

-   Fixed api migration bug #2087

# 24.6.2

### Bug Fixes

-   Remove default config value for advanced signatures #2090

# 24.6.1

### Features

-   Add rate limiter to event entry points, HTTP APIs, and message brokers #2072 #2035
-   Add retry queue #2058
-   Add search bar for subscriptions #2062
-   Use asynq's exponential backoff for event delivery retries #2052
-   Add PyroScope profiling #1737
-   Added Prometheus metrics #2005
-   Endpoint subscriptions can now be created and viewed on portal links #2015
-   Subscriptions can now be filtered by name #2014

### Enhancements

-   Optimise flatten and compare packages #2066 #2077
-   Changed the Postgres driver to pgx #2064
-   Optimized dashboard query and run time #2070
-   Fixed order of endpoints and subscriptions #2060
-   Optimise Subscription loader queries #2056
-   Make dispatcher proxy less strict, allowing requests to be sent if the proxy URL is invalid #2059
-   Load subscriptions synchronously on worker startup #2053
-   Remove cache from userRepo since it's not a hot path #2050
-   Added cache to retrieving subscriptions for broadcast #2044
-   Refactored net.Dispatcher
    -   Create dispatcher once #2043
    -   Fixed a bug where we were closing the response body late #2029
-   Refactor ProcessBroadcastEventCreation and ProcessEventCreation handlers
    -   Deduplicate endpoint IDs #2024
-   Refactor Repository
    -   Add FindEventDeliveryByIDSlim #2054
    -   Refactored event creation to insert in bulk #2038
    -   Add indexes for FetchSubscriptionsForBroadcast #2033

### Bug Fixes

-   Fix dashboard summary filters #2071
-   Fix events summary date filter #2069
-   Fixed panic that occurred when updating project when setting the SSL config #2055
-   Fixed a bug where we were creating events for the number of matched subscriptions instead of the number of matched endpoints #2025
-   Fixed a bug where we would try to filter even when none is set on the subscription #2027
-   Fixed a bug where the default log level was set to debug #2013
-   Fixed a bug where portal links won't load in iframes because of a missing project reference #2008

# 24.5.1

### Features

-   Create default org when bootstrapping Convoy for the first time. #1991
-   Display user-settings page when there are no organizations. #1999

### Enhancements

-   Use transactions in ProcessBroadcastEventCreation to prevent a race condition. #1994
-   Update copy for the kafka source form giving more information and linking to docs. #2000
-   Update Endpoint table with its ID on the dashboard. #1988

### Bug fixes

-   Fixed a panic that would occur when request body is a string during subscription filtering. #1992
-   Fixed a bug where the response from the pause endpoint api wasn't versioned correctly. #2001

# 24.4.1

### API Changes

> [!NOTE]
> All API Changes are backward-compatible, so you shouldn't need to change any code to get them to work, however, you need to specify the version (2024-04-01) in your convoy.json.

-   changed endpoint `title` to `name`
-   changes endpoint `target_url` to `url`

### Features

-   Implemented an in-memory store for data plane #1932
-   Re-implement rate limiter using postgres #1937 #1950
-   Add the ability to mutate payloads from message broker sources using javascript functions #1954 #1956 #1958
-   Add project config for enforcing https endpoints #1955 #1957
-   Add documentation to request models #1959
-

### Enhancements

-   Encode Postgres connection string credentials #1936
-   Update endpoint `title` to `name` and `target_url` to `url` #1945
-   Enqueue Stuck Event Deliveries #1977

### Bug Fixes

-   Fixed a bug where telemetry wasn't being sent to PostHog #1944
-   Fixed a bug where the signature modal in the project settings doesn't dismiss after saving. #1939
-   Fixed a bug where project settings were not displayed properly on the dashboard #1953
-   Fixed a bug where a failed subscription filter will stop all subscribers from a broadcast event from receiving the event #1962
-   Fixed open telemetry tls configuration #1966
-   Fixed a bug where a created or updated subscription didn't show the nested values #1970
-   Fixed endpoints count query for portal links #1973
-   Added data plane capabilities back to the worker which was unintentionally removed #1974

# 24.1.4

-   [Enhancement] Add custom headers to dynamic event #1923
-   [Feature] Add broadcast event api #1913

# 24.1.3

### Bug Fixes

-   fixed api versioning bug to correctly retrieve the instance api version #1918

# 24.1.2

### Bug Fixes

-   fixed sync bug in the oss telemetry library #1906

# 24.1.1

### API Changes

> [!NOTE]
> All API Changes are backward-compatible, so you shouldn't need to change any code to get them to work, however, you need to specify which version you're running in your convoy.json.

-   changed `http_timeout` and `rate_limit_duration` in endpoints from duration string to `int`.
-   changed the default signature format from `simple` to `advanced`.
-   stripped out unnecessary fields from dynamic api endpoint.

### Features

-   added support for OpenTelemetry #1865
-   added support for sentry as a tracing backend #1865
-   added support for api versioning using rolling versioning strategy backwards compatible #1871

### Bug Fixes

-   added `eventType` to `QueryListEventDelivery` #1843
-   fixed source and subscription forms #1876
-   fixed source and endpoint dropdown with search box #1850
-   fixed retrieving portal links by `endpoints` or `owner_id` #1894
-   update endpoints in cache when all the endpoints are re-enabled #1847
-   update subscription endpoint metadata when a subscription is updated #1891
-   fixed event deliveries pagination #1846
-   delete invite after cancellation #1860
-   enabled multi-user invite without refresh #1861
-   set `event_id` in events filter #1866

### Enhancements

-   improved dynamic api support #1884
-   improved endpoints api #1870
-   QoL improvements to the api layer #1851
-   QoL improvements to retention policies export worker #1882

# 23.11.1

-   [Feature] add cache to the organisations and api key repositories, add profiling route #1822
-   [Feature] Record event delivery latency #1830
-   [Enhancement] Improve event deliveries filtering #1824 #1840
-   [Enhancement] UI layout redesign #1815
-   [Enhancement] Move scheduler functionality into server #1835
-   [Bugfix] Fixed endpoint enabling and disabling #1837

# 23.10.1

-   [Feature] Added bootstrap cli for user account creation #1773
-   [Feature] Add prefix configuration to S3 Config #1812
-   [Enhancement] Added TLS option for SMTP config #1784
-   [Enhancement] Added support for multi-architecture docker images #1790
-   [Enhancement] Improved docker compose to use named volumes #1804
-   [Enhancement] Replaced Flipt for a custom feature flag implementation #1797
-   [Enhancement] Added several performance improvements with caches and reduced db calls #1765 #1783
-   [Enhancement] Optimise source loader query #1806
-   [Bugfix] Added separate port for `ingest` cli #1795
-   [Bugfix] Add support for Idempotency keys in message broker integration #1800
-   [Bugfix] Fixed concurrency bug where wrong source name is show in the event log #1800
-   [Bugfix] Fixed role check for updating organization name #1805
-   [Bugfix] Fixed a bug with the portal link delete button #1807
-   [Bugfix] Fixed a bug with the endpoint config button #1810
-   [Bugfix] Removed onclickout function for dialogs #1808
-   [Bugfix] Generate exponential back-off rate limits from intervalSeconds and Limt #1813

# 23.9.2

-   [Enhancement] Show invite url on teams invite page
-   [Bugfix] Handle nullable 'Function' field in worker handler

# 23.09.1

-   [Feature] Add event payload transform functionality #1755 #1761
-   [Enhancement] Add tail mode for events and event deliveries #1753
-   [Enhancement] Expose rate limiting for endpoints #1754
-   [Bugfix] Use the different queue instance when starting stream server #1769
-   [Bugfix] Return an appropriate error instead of nil the process event delivery #1756
-   [Bugfix] Add permissions when creating and revoking API keys #1762
-   [Bugfix] Add QueueUrl nil check in SQS handler #1763
-   [Bugfix] Update endpoints migration query #1768

# 23.08.2

-   [Feature] Postgres Full Text Search Reimplementation #1734 #1751 #1750
-   [Feature] Add tail mode for events and event deliveries #1753
-   [Enhancement] Paused events polling when searching and filtering on the Event Log #1744
-   [Enhancement] Added an edit endpoint button in event delivery page #1738
-   [Enhancement] Added a tooltip for Retry and Force Retry buttons #1741
-   [Bugfix] Fixed a bug where the subscription filter editor UI was unresponsive #1747
-   [Bugfix] Fixed a bug where the Batch Replay button on the events log would not replay events #1740
-   [Bugfix] Fixed a bug in the process event delivery handler that caused events to stay in the `Scheduled` state #1756

# 23.08.1

-   [Feature] Check if signup is enabled in the instance config with this new API #1710
-   [Feature] New support added for Redis clusters #1700
-   [Feature] Add healthcheck for ingest command #1709
-   [Feature] New subscription filter based on regex #1725
-   [Feature] Integration added for Kafka sources #1708
-   [Enhancement] Add source ID header to ingested events #1715
-   [Enhancement] Get event deliveries based on subscription ID #1717
-   [Enhancement] Made improvements to Convoy's UI Modals #1711
-   [Enhancement] Display event types on event deliveries table #1691
-   [Enhancement] Use of a separate struct when building meta events to preserve the event delivery attempts #1693
-   [Enhancement] Update how events and event deliveries are fetched regular intervals #1705
-   [Enhancement] Prevent other non-server entry points from modifying instance config #1724
-   [Enhancement] Ensure that endpoint titles are unique #1730
-   [Bugfix] Resolved a console error when creating a source #1690
-   [Bugfix] Display of event types on the portal page. #1692
-   [Bugfix] Fixed Endpoints page loading state and other issues #1697
-   [Bugfix] Fixed portal link card spacing, scroll and token timeout issues #1707
-   [Bugfix] Resolved memory consumption issues when the retention policy job runs #1706
-   [Bugfix] Fixed issues encountered during onboarding related to subscriptions #1713
-   [Bugfix] The copy button on the project details page has been fixed #1722
-   [Bugfix] Events with active deliveries are now ignored in retention policies #1723
-   [Bugfix] Fixed an issue where the organization modal fails to open and the project page keeps loading after creating the first organization #1727

# 23.06.3

-   [Feature] Add support for Webhooks Idempotency #1651 #1688
-   [Enhancement] Add support for deleting archived tasks #1657
-   [Enhancement] Improved list projects view #1669 #1678 #1683
-   [Enhancement] Remove 50Kb limit on MaxResponseSize config #1675
-   [Enhancement] Create Fanout Event if Owner ID is tied to a portal link #1682
-   [Bugfix] Delete duplicate task ID when writing to queue #1660
-   [Bugfix] Fixed view endpoint under portal link's event delivery page #1666
-   [Bugfix] Fixed issue with closing google pub/sub client #1673
-   [Bugfix] Fixed issue with overriding config with cli flags #1668
-   [Bugfix] Fixed issue with HTTP timeout validation #1680

# 23.06.2

-   [Enhancement] Improved logging to include response body #1655
-   [Enhancement] Improved Datetime filtering UX #1644
-   [Bugfix] Fixed update project settings while switching tabs bug #1653
-   [Bugfix] Fixed default retention policy #1652
-   [Bugfix] Fix multi-tenancy issue on portal links with ownerID. #1654

# 23.06.1

-   [Feature] Add custom response to incoming project sources #1605
-   [Feature] Enabled endpoint management on portal links #1616
-   [Feature] Support ingest query parameters to incoming sources #1640
-   [Enhancement] Improved New Relic Integration #1621
-   [Enhancement] Enabled auto API docs syncing #1625
-   [Enhancement] Add endpoint timeout config option to the dashboard #1614
-   [Enhancement] Improved portal page responsiveness #1648
-   [Enhancement] Improved request and response annotations #1608 #1619 #1627 #1622 #1630 #1611 #1636 #1632 #1637 #1631 #1641
-   [Bugfix] Fixed FindSubscriptionByDeviceID query bug #1647
-   [Bugfix] Coalesce url query parameters #1642
-   [Bugfix] Fixed event delivery filtering by status #1626
-   [Bugfix] Fixed index on organisations invite index #1603 #1607
-   [Bugfix] Link ownerID to portal links without endpoints #1638

# 23.05.5

-   [Enhancement] Optimise Migration Queries #1601

# 23.05.4

-   [Bugfix] Return error when persisting to redis fails on the ingest route #1597

# 23.05.3

-   [Enhancement] Postgres and Redis config options will now be supplied in parts to allow for fine-grained configuration #1579
-   [Bugfix] Fixed an issue where the email verification flow could not be completed #1586
-   [Bugfix] Fixed an issue where the frontend client sent the wrong pagination cursor value #1588
-   [Bugfix] Fixed an issue where a project could not be saved due to meta-event form valiation #1589
-   [Bugfix] Fixed an issue where the source id query param was not being used to filter when fetching events #1587

# 23.05.2

-   [Feature] Added support for meta events #1541
-   [Bugfix] Fixed multi-tenancy bug with portal links #1582
-   [Bugfix] Fixed issue with new user with no organisation #1578

# 23.05.1

-   [Feature] Add support for on the fly events #1558
-   [Enhancement] Add prompt for disabling endpoints #1556
-   [Enhancement] Extend subscription filter capabilities #1545 #1566
-   [Enhancement] Improved logging for all workers #1560

# 0.9.2

-   [Documentation] Fix noun/verb agreement #1504
-   [Enhancement] Add support for pausing an endpoint #1529 #1527
-   [Bugfix] Fixed a bug where an endpoint would be stuck in pending #1529
-   [Bugfix] Fixed max response size log on server startup #1507
-   [Bugfix] Add endpoint metadata to event search results #1508
-   [Bugfix] Fixed an issue where the reset password flow could not be completed #1506 #1503

# 0.9.1

-   [Enhancement] Add default db connection options #1496
-   [Enhancement] Move check for hiding pagination to entire container #1497
-   [Enhancement] Add support for filtering by "owner_id" when fetching endpoints and by an array of endpoints when fetching subscriptions #1498
-   [Enhancement] Add signup enabled environment variable #1495
-   [Bugfix] Change TrimSuffix to TrimSpace when fetching convoy version #1501
-   [Bugfix] Redirect to the "get started" page when there's no orgnaization for that user #1500

# 0.9.0

-   [Documentation] Update API Annotations #1478
-   [Enhancement] Change font to inter #1470
-   [Enhancement] QoL Postgres Updates #1419
-   [Enhancement] Port Stream Server #1482
-   [Enhancement] UI updates #1491 #1490 #1486 #1474 #1480
-   [Bugfix] Check pending migrations #1487
-   [Bugfix] fix message payload for process event delivery #1483

# 0.9.0-rc.3

-   [Bugfix] Fix search indexer job #1448 #1449
-   [Bugfix] Fix refresh token issue #1447
-   [Bugfix] Fix event graph length #1443

# 0.9.0-rc.2

-   [Bugfix] Fix issue with updating PubSub Sources #1440
-   [Enhancement] Fix toggle and view subscription. #1424
-   [Enhancement] Create room for empty data on chart. #1422 #1425

# 0.9.0-rc.1

-   [Feature] Add new message broker source. #1285
-   [Enhancement] Switched backing store to PostgreSQL. #1287
-   [Enhancement] Add modal to confirm before regenerating API Keys. #1378
-   [Enhancement] Implement new wait screens #1398

# 0.8.3

-   [Bugfix] Fix search indexer job #1448 #1449

# 0.8.2

-   [Feature] Set Advanced Signatures by default for Incoming projects.
-   [Enhancement] Refactored Convoy CLI Implementation. #1290
-   [Enhancement] Improved Project Onboarding #1335
-   [Enhancement] Improved Delivery Attempt Details page #1341
-   [Enhancement] Update Subscriptions Card for Incoming Projects. #1349
-   [Enhancement] Update portal links design #1348
-   [Enhancement] Improve source, subscription and endpoint forms #1335
-   [Bugfix] Event Log pagination bug #1346
-   [Bugfix] Date picker bug fix #1355

# 0.8.1

-   [Enhancement] Improved stats query #1256
-   [Enhancement] Add analytics and storage policy settings in convoy.josn #1284 #1324
-   [Feature] Set notifications by default #1251
-   [Feature] Add raw invite link in invite email #1279
-   [Feature] Allow user re-generate api key for projects #1277

# 0.8.0

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

# 0.7.6

-   [Bugfix] Preserve whitespaces in event payload #1223

# 0.7.5

-   [Enhancement] Filter event deliveries by subscriptions #1192
-   [Bugfix] fix prism import error #1194
-   [Bugfix] correctly use query string in app portal #1195

# 0.7.4

-   [Enhancement] hide cli keys and devices on app portal #1184
-   [Bugfix] pass query params along in shim #1185

# 0.7.3

-   [Enhancement] Set app portal key expiration to 30 days #1170 #1171

# 0.7.2

-   [Bugfix] Fix Max response size CLI override #1098
-   [Feature] Advanced Signatures Core Implementation #1053 #1100 #1103 #1104
-   [Feature] Refactored Logging Implementation #1051
-   [Bugfix] Update endpoint with custom authentication #1119 #1106 #1107 #1105
-   [Enhancement] Add support for building alpine image #1108

# 0.7.1

-   [Bugfix] Fix Max response size CLI override #1098

# 0.7.0

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

# 0.7.0-rc.2

-   [Feature] Create cli api keys and manage devices from the app portal #983
-   [Bugfix] Fix Subscription Config Inheritance For Event Deliveries #995
-   [Enhancement] Implement cascading delete for groups, apps and sources #1037
-   [Enhancement] Add help to project sidebar #1032

# 0.7.0-rc.1

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

# 0.6.8

-   [Bugfix] Fix subscriptions page error #1026
-   [Bugfix] Add deleted_at to MongoDB indexes #1027
-   [Bugfix] Dismiss modal when a source is deleted #1026

# 0.6.7

-   [Bugfix] Fix analytics query #1001
-   [Bugfix] Fix middleware layer for app portal #988
-   [Bugfix] Fix endpoint notification email bug #981

# 0.6.6

-   [Bugfix] Fix project statistics lookup query #978 #979

# v0.6.5

-   [Feature] Add support for user registration. #959
-   [Bugfix] Updated event dashboard with application and source metadata. #960
-   [Bugfix] Allow re-invitation of a previously cancelled invitation. #967
-   [Enhancement] Projects scoped indexing and search. #945
-   [Enhancement] Improved notification system. #954

# v0.6.4

-   [Bugfix] Fixed a bug where event deliveries could not be force retried #938
-   [Bugfix] Changed the group/project rate limit duration type from string to int #938
-   [Bugfix] Fixed a bug where team invites could not be canceled on the UI #940
-   [Enhancement] Add an environment variable for the typesense collection name #939

# v0.6.3

-   [Enhancement] change app portal url from `/app-portal` to `/app` #924
-   [Enhancement] App portal improvements #918

# v0.6.2

-   [Feature] Added Proxy Events without Verification #906
-   [Enhancement] Reliably forward Incoming events request headers #895
-   [Bugfix] Fixed force retry bug #891 #890

# v0.6.1

-   [Bugfix] Fixed API response for force retry endpoint #892, #897
-   [Bugfix] Changed create configuration to use a post request #896

# v0.6.0

-   [Feature] Add Support for Custom Sources (Twitter, Shopify) #869, #833, #826
-   [Feature] Add Support for Retention Policies #839, #879
-   [Enhancement] Updated UI Architecture to use Tailwind CSS #816
-   [Enhancement] Optimised UI to reduce initial bundle size #879
-   [Enhancement] Allow all workers to run in a single cluster #876
-   [Enhancement] Add cancelled status on org invite #812
-   [Bugfix] Fixed wrong FindMany query in subscriptions.go #858
-   [Bugfix] Enabled JWT configuration with env variables #813

# v0.6.0-rc.4

-   [Bugfix] Fixed bug in correctly using datastore FindMany method #856

# v0.6.0-rc.3

-   [Change] Changed host to instance id in the analytics #821
-   [Enhancement] Add support for JWT environment variables #813
-   [Bugfix] Fix analytics query #825
-   [Bugfix] UI bug fixes #814

# v0.6.0-rc.2

-   [Bugfix] Dereference slice when finding source subscriptions #808
-   [Bugfix] Use redis client in scheduler #807
-   [Bugfix] Fixed an issue where the source type would not be updated when updating a Github source
-   [Bugfix] Fixed an issue where the application details would not be loaded when creating a subscription
-   [Bugfix] Fixed an issue where an organization created by a user would not show on the top bar to be selected

# v0.6.0-rc.1

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

# v0.6.0-rc

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

# v0.5.3

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

# v0.5.0

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

# 0.4.10

-   [Feature] We can now download convoy binaries from package managers #459
-   [Enhancement] Add support for embedding convoy version file #454
-   [Feature] Expose taskq queue metrics #476
-   [Feature] Added support for embedding an App portal in a 3rd pary app #463

# 0.4.0

-   [Enhancement] Replace bbolt implementation with badger #417
-   [Enhancement] Add compound indexex for events, eventdeliveries and applications #418

# 0.4.0-rc

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

# 0.3.0

-   [Bugfix] Fixed hash computation bug #269
-   [Bugfix] Bundled UI into binary builds. #273
-   [Bugfix] Don't enqueue discarded events #253
-   [Enhancement] Build Convoy dashboard UI into npm package in `dashboard-component` #272

# 0.3.0-rc

-   [Feature] URL/Events: User can specify events to each endpoint.
-   [Feature] Batch Retries: User can batch retry multiple event deliveries at once.
-   [Feature] Unified auth configuration for both UI and API.
-   [Feature] Added minimal rbac for super user, admin and ui admin.
-   [Feature] New tab to view event deliveries for events.
-   [Feature] Filter event deliveries by delivery status, app and date range
-   [Feature] View event deliveries status for each event from event's details section.
-   [Feature] Introduced Groups: To support multi-tenancy for multiple products to pipe events as separete groups.
-   [Feature] Persist events and event deliveries filters, active group and active logs tab with page reload.

# 0.2.0

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
