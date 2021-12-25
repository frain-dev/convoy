## 0.3.0
* [Bugfix] Fixed hash computation bug #269
* [Bugfix] Bundled UI into binary builds. #273
* [Bugfix] Don't enqueue discarded events #253
* [Enhancement] Build Convoy dashboard UI into npm package in `dashboard-component`

## 0.3.0-rc
* [Feature] URL/Events: User can specify events to each endpoint.
* [Feature] Batch Retries: User can batch retry multiple event deliveries at once.
* [Feature] Unified auth configuration for both UI and API.
* [Feature] Added minimal rbac for super user, admin and ui admin.
* [Feature] New tab to view event deliveries for events. 
* [Feature] Filter event deliveries by delivery status, app and date range
* [Feature] View event deliveries status for each event from event's details section. 
* [Feature] Introduced Groups: To support multi-tenancy for multiple products to pipe events as separete groups.
* [Feature] Persist events and event deliveries filters, active group and active logs tab with page reload.

## 0.2.0
* [Feature] Add disable events and send email notifications.
* [Feature] Re-activate endpoints by re-trying a non-successful event.
* [Feature] Enable SMTP configuration.
* [Enhancement] Improved Delivery Attempt Page.
* [Enhancement] Event log filtering by Applications and Date.
* [Enhancement] Changed organisations to groups throughout app.
* [Enhancement] Changed /apps to /applications
* [Enhancement] Create default group on app startup.
* [Enhancement] Clicking events button from apps table now automatically filters events by clicked app.
* [Enhancement] Convoy config details now shows on dashboard.
* [Enhancement] Created at and Next retry on events table now shows time instead of date.
* [Enhancement] Improved table pagination.
* [Enhancement] Events table now grouped by date.
* [Enhancement] Manually retried events now identifiable by a retry icon on events table.
* [Enhancement] Event status now differentiated by status color.
