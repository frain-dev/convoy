<script>
import { defineComponent } from 'vue';
import moment from 'moment';
import Prism from 'prismjs';
import 'prismjs/themes/prism.css';
import 'prismjs/plugins/line-numbers/prism-line-numbers';
import DatePicker from 'vue-datepicker-next';
import 'vue-datepicker-next/index.css';
import { format } from 'date-fns';
import LoaderComponent from './loader-component.vue';
import SvgComponent from './svg-component.vue';

export default /*#__PURE__*/ defineComponent({
	name: 'ConvoyApp', // vue component name
	props: ['token', 'apiURL'],
	components: { DatePicker, LoaderComponent, SvgComponent },
	data() {
		return {
			appDetails: {},
			tabs: ['events', 'event deliveries'],
			activeTab: 'events',
			events: { content: [], pagination: {} },
			eventsPage: 1,
			isloadingMoreEvents: false,
			isloadingEvents: false,
			eventDetailsActiveTab: 'data',
			eventDeliveries: { content: [], pagination: {} },
			eventDeliveriesPage: 1,
			isloadingEventDeliveries: false,
			isloadingMoreEventDeliveries: false,
			eventsDetailsItem: {},
			eventDeliveryFilteredByStatus: [],
			eventDeliveriesStatusFilterActive: false,
			eventDeliveryFilteredByEventId: '',
			eventDelsDetailsItem: {},
			isloadingDeliveryAttempts: false,
			eventDeliveryAtempt: {},
			sidebarEventDeliveries: [],
			eventDetailsTabs: [
				{ id: 'data', label: 'Event' },
				{ id: 'response', label: 'Response' },
				{ id: 'request', label: 'Request' }
			],
			eventDeliveriesDateFilter: [],
			eventsDateFilter: [],
			showEventDelsStatusDropdown: false,
			allEventDeliveryStatus: ['Success', 'Failure', 'Retry', 'Scheduled', 'Processing', 'Discarded'],
			loadingAppDetails: false
		};
	},
	computed: {
		displayedEvents: {
			get() {
				return this.setEventsDisplayed(this.events?.content);
			},
			set(value) {
				this.events = value;
			}
		},
		displayedEventDeliveries: {
			get() {
				return this.setEventsDisplayed(this.eventDeliveries?.content);
			},
			set(value) {
				this.eventDeliveries = value;
			}
		},
		_eventsDetailsItem: {
			get() {},
			set(eventDetails) {
				this.eventsDetailsItem = eventDetails;
			}
		},
		_eventDelsDetailsItem: {
			get() {},
			set(eventDetails) {
				this.eventDelsDetailsItem = eventDetails;
			}
		},
		_eventDeliveryFilteredByEventId: {
			get() {},
			set(eventDeliveryFilterId) {
				this.eventDeliveryFilteredByEventId = eventDeliveryFilterId;
			}
		}
	},
	mounted() {
		Promise.all([this.getAppDetails()]).then(() => {
			window.Prism = window.Prism || {};
			window.Prism.manual = true;
			Prism.highlightAll();
		});
	},
	methods: {
		date(value) {
			return moment(String(value)).format('MMMM DD, YYYY');
		},
		time(value) {
			return moment(String(value)).format('h:mm:ss a');
		},
		toggleActiveTab(tab) {
			this.activeTab = tab;

			if (tab === 'events' && this.events?.content?.length > 0) {
				if (!this.eventsDetailsItem) this._eventsDetailsItem = this.events?.content[0];
				if (this.eventsDetailsItem?.uid) this.getEventDeliveriesForSidebar(this.eventsDetailsItem.uid);
			} else if (tab === 'event deliveries' && this.eventDeliveries?.content?.length > 0) {
				if (!this.eventDelsDetailsItem) this.eventDelsDetailsItem = this.eventDeliveries?.content[0];
				if (this.eventDelsDetailsItem?.uid) this.getDelieveryAttempts(this.eventDelsDetailsItem.uid);
			}
		},
		setDateForFilter(dates) {
			if (!dates?.endDate && !dates?.startDate) return { startDate: '', endDate: '' };
			const startDate = dates.startDate ? `${format(dates.startDate, 'yyyy-MM-dd')}T00:00:00` : '';
			const endDate = dates.endDate ? `${format(dates.endDate, 'yyyy-MM-dd')}T23:59:59` : '';
			return { startDate, endDate };
		},
		request({ url, method, body }) {
			return new Promise(async (resolve, reject) => {
				try {
					const response = await fetch(this.apiURL + '/portal' + url, {
						method,
						body,
						headers: {
							Authorization: `Bearer ${this.token}`,
							'Content-Type': 'application/json'
						},
						mode: 'cors'
					});
					resolve(response.json());
				} catch (error) {
					reject(error);
				}
			});
		},
		async getAppDetails() {
			this.loadingAppDetails = true;
			try {
				const appDetailsResponse = await this.request({
					url: `/apps`,
					method: 'get'
				});

				this.appDetails = appDetailsResponse.data;
				this.getEventDeliveries();
				this.getEvents();
				this.loadingAppDetails = false;
			} catch (error) {
				this.loadingAppDetails = false;
				return error;
			}
		},
		setEventsDisplayed(events) {
			const dateCreateds = events?.map(event => this.date(event.created_at));
			const uniqueDateCreateds = [...new Set(dateCreateds)];
			const displayedEvents = [];
			uniqueDateCreateds.forEach(eventDate => {
				const filteredEventDate = events.filter(event => this.date(event.created_at) === eventDate);
				const eventsItem = { date: eventDate, events: filteredEventDate };
				displayedEvents.push(eventsItem);
			});
			return displayedEvents;
		},
		async getEvents() {
			this.events?.pagination?.next === this.eventsPage ? (this.isloadingMoreEvents = true) : (this.isloadingEvents = true);

			const { startDate, endDate } = this.setDateForFilter({ startDate: this.eventsDateFilter[0], endDate: this.eventsDateFilter[1] });

			try {
				const eventsResponse = await this.request({
					url: `/events?appId=${this.appDetails.uid || ''}&sort=AESC&page=${this.eventsPage || 1}&perPage=20&startDate=${startDate}&endDate=${endDate}`,
					method: 'get'
				});

				if (this.events && this.events?.pagination?.next === this.eventsPage) {
					const content = [...this.events.content, ...eventsResponse.data.content];
					const pagination = eventsResponse.data.pagination;
					this.displayedEvents = { content, pagination };
					this.isloadingMoreEvents = false;
					return;
				}

				this.displayedEvents = eventsResponse.data;

				this._eventsDetailsItem = this.events?.content[0];
				this.getEventDeliveriesForSidebar(this.eventsDetailsItem.uid);

				this.isloadingEvents = false;
			} catch (error) {
				this.isloadingEvents = false;
				this.isloadingMoreEvents = false;
				return error;
			}
		},
		async eventDeliveriesRequest(requestDetails) {
			let eventDeliveryStatusFilterQuery = '';
			this.eventDeliveryFilteredByStatus.length > 0 ? (this.eventDeliveriesStatusFilterActive = true) : (this.eventDeliveriesStatusFilterActive = false);
			this.eventDeliveryFilteredByStatus.forEach(status => (eventDeliveryStatusFilterQuery += `&status=${status}`));

			try {
				const eventDeliveriesResponse = await this.request({
					url: `/eventdeliveries?appId=${this.appDetails.uid || ''}&eventId=${requestDetails.eventId || ''}&page=${this.eventDeliveriesPage || 1}&startDate=${requestDetails.startDate}&endDate=${
						requestDetails.endDate
					}${eventDeliveryStatusFilterQuery || ''}`,
					method: 'get'
				});

				return eventDeliveriesResponse;
			} catch (error) {
				return error;
			}
		},
		async getEventDeliveries() {
			this.eventDeliveries && this.eventDeliveries?.pagination?.next === this.eventDeliveriesPage ? (this.isloadingMoreEventDeliveries = true) : (this.isloadingEventDeliveries = true);
			const { startDate, endDate } = this.setDateForFilter({ startDate: this.eventDeliveriesDateFilter[0], endDate: this.eventDeliveriesDateFilter[1] });

			try {
				const eventDeliveriesResponse = await this.eventDeliveriesRequest({
					eventId: this.eventDeliveryFilteredByEventId,
					startDate,
					endDate
				});

				if (this.eventDeliveries && this.eventDeliveries?.pagination?.next === this.eventDeliveriesPage) {
					const content = [...this.eventDeliveries.content, ...eventDeliveriesResponse.data.content];
					const pagination = eventDeliveriesResponse.data.pagination;
					this.displayedEventDeliveries = { content, pagination };
					this.isloadingMoreEventDeliveries = false;
					return;
				}

				this.displayedEventDeliveries = eventDeliveriesResponse.data;

				this.eventDelsDetailsItem = this.eventDeliveries?.content[0];
				this.getDelieveryAttempts(this.eventDelsDetailsItem.uid);

				this.isloadingEventDeliveries = false;
				return eventDeliveriesResponse.data.content;
			} catch (error) {
				this.isloadingEventDeliveries = false;
				this.isloadingMoreEventDeliveries = false;
				return error;
			}
		},
		async getEventDeliveriesForSidebar(eventId) {
			Prism.highlightAll();
			const response = await this.eventDeliveriesRequest({ eventId, startDate: '', endDate: '' });
			this.sidebarEventDeliveries = response.data.content;
			Prism.highlightAll();
		},
		async getDelieveryAttempts(eventDeliveryId) {
			this.isloadingDeliveryAttempts = true;

			try {
				const deliveryAttemptsResponse = await this.request({
					url: `/eventdeliveries/${eventDeliveryId}/deliveryattempts`,
					method: 'get'
				});
				this.eventDeliveryAtempt = deliveryAttemptsResponse.data[deliveryAttemptsResponse.data.length - 1];
				this.isloadingDeliveryAttempts = false;

				setTimeout(() => {
					Prism.highlightAll();
				}, 100);

				return;
			} catch (error) {
				this.isloadingDeliveryAttempts = false;
				return error;
			}
		},
		async openDeliveriesTab() {
			await this.getEventDeliveries();
			delete this.eventsDelsDetailsItem;
			this.toggleActiveTab('event deliveries');
		},
		updateEventDevliveryStatusFilter(status, isChecked) {
			if (isChecked.target.checked) {
				this.eventDeliveryFilteredByStatus.push(status);
			} else {
				let index = this.eventDeliveryFilteredByStatus.findIndex(x => x === status);
				this.eventDeliveryFilteredByStatus.splice(index, 1);
			}
		},
		checkIfEventDeliveryStatusFilterOptionIsSelected(status) {
			return this.eventDeliveryFilteredByStatus?.length > 0 ? this.eventDeliveryFilteredByStatus.includes(status) : false;
		},
		async retryEvent(requestDetails) {
			requestDetails.e.stopPropagation();
			const retryButton = document.querySelector(`#event${requestDetails.index} button`);
			if (retryButton) {
				retryButton.classList.add(['spin', 'disabled']);
				retryButton.disabled = true;
			}

			try {
				await this.request({
					method: 'put',
					url: `/eventdeliveries/${requestDetails.eventDeliveryId}/resend`
				});

				// pending when I add the notification component
				// this.convyAppService.showNotification({
				// 	message: 'Retry Request Sent'
				// });
				retryButton.classList.remove(['spin', 'disabled']);
				retryButton.disabled = false;
				this.getEventDeliveries();
			} catch (error) {
				// pending when I add the notification component
				// this.convyAppService.showNotification({
				// 	message: error.error.message
				// });
				retryButton.classList.remove(['spin', 'disabled']);
				retryButton.disabled = false;
				return error;
			}
		}
	}
});
</script>

<template>
	<div class="dashboard--page">
		<div class="dashboard--page--head">
			<h3 class="margin-bottom__10px">Endpoints</h3>
		</div>

		<div class="dashboard-page--details">
			<div class="card has-title dashboard-page--endpoints has-loader">
				<LoaderComponent v-if="loadingAppDetails" />

				<table class="table table__no-style">
					<thead>
						<tr class="table--head">
							<th class="has-long-text" scope="col">Endpoint URL</th>
							<th scope="col">Created At</th>
							<th scope="col">Updated At</th>
							<th scope="col">Endpoint Events</th>
							<th scope="col">Status</th>
						</tr>
					</thead>

					<tbody>
						<tr class="has-border" v-for="(endpoint, index) in appDetails?.endpoints" :key="index">
							<td class="has-long-text longer">
								<div>{{ endpoint.target_url }}</div>
							</td>
							<td>
								<div>{{ date(endpoint.created_at) }}</div>
							</td>
							<td>
								<div>{{ date(endpoint.updated_at) }}</div>
							</td>
							<td>
								<div class="flex flex__wrap">
									<div class="tag" v-for="(event, index2) in endpoint.events" :key="index2">{{ event == '*' ? 'all events' : event }}</div>
								</div>
							</td>
							<td>
								<div>
									<div :class="'tag' + (endpoint.status === 'active' ? ' tag--Success' : '')">{{ endpoint.status }}</div>
								</div>
							</td>
						</tr>
					</tbody>
				</table>

				<p class="empty-table" v-if="appDetails && appDetails?.endpoints > 0">No endpoint has been add for this app yet</p>
			</div>
		</div>

		<section class="card has-title dashboard--logs">
			<div class="dashboard--logs--tabs">
				<div class="tabs">
					<li v-for="(tab, index) in tabs" @click="toggleActiveTab(tab)">
						<button :class="activeTab === tab ? 'active' : ''">
							<span>{{ tab }}</span>
						</button>
					</li>
				</div>
			</div>

			<div class="dashboard--logs--filter">
				<div class="flex flex__align-items-center flex__justify-between" v-show="activeTab === 'events'">
					<div class="flex flex__align-items-center">
						<date-picker
							class="button__filter"
							:class="{ active: eventsDateFilter.length > 0 }"
							@change="getEvents()"
							v-model:value="eventsDateFilter"
							type="date"
							range
							placeholder="Select date range"
						></date-picker>
					</div>

					<button
						class="button button__white button__small margin-right__20px"
						:disabled="eventsDateFilter.length === 0"
						@click="
							eventsDateFilter = [];
							getEvents();
						"
					>
						Clear Filter
					</button>
				</div>

				<div class="flex flex__align-items-center flex__justify-between" v-show="activeTab === 'event deliveries'">
					<div class="flex flex__align-items-center">
						<date-picker
							class="button__filter"
							:class="{ active: eventDeliveriesDateFilter.length > 0 }"
							@change="getEventDeliveries()"
							v-model:value="eventDeliveriesDateFilter"
							type="date"
							range
							placeholder="Select date range"
						></date-picker>

						<div class="dropdown">
							<button
								class="button button__filter margin-left__24px"
								:class="{ active: eventDeliveryFilteredByStatus.length > 0 }"
								@click="showEventDelsStatusDropdown = !showEventDelsStatusDropdown"
							>
								<SvgComponent :width="'16'" :height="'14'" :id="'status-icon'" :styles="'margin-top__4px'" />
								<span>Status</span>
								<SvgComponent :width="'12'" :height="'8'" :id="'angle-arrow-down'" />
							</button>

							<div class="dropdown__menu with-padding small" :class="{ show: showEventDelsStatusDropdown }">
								<div class="dropdown__menu__item with-border" v-for="(status, index) in allEventDeliveryStatus" :key="index">
									<label :for="status">{{ status }}</label>
									<input
										type="checkbox"
										name="status"
										:value="status"
										:id="status"
										@change="updateEventDevliveryStatusFilter(status, $event)"
										:checked="checkIfEventDeliveryStatusFilterOptionIsSelected(status)"
									/>
								</div>

								<div class="flex flex__align-items-center margin-top__12px">
									<button
										class="button button__primary button__small"
										@click="
											getEventDeliveries();
											showEventDelsStatusDropdown = false;
										"
									>
										Apply
									</button>
									<button
										class="button__clear margin-left__14px"
										@click="
											eventDeliveryFilteredByStatus = [];
											showEventDelsStatusDropdown = false;
											getEventDeliveries();
										"
									>
										Clear
									</button>
								</div>
							</div>
						</div>

						<div class="button__filter active margin-left__24px" v-show="this.eventDeliveryFilteredByEventId">
							Event Filtered
							<button
								class="button__clear button--has-icon margin-left__8px"
								@click="
									eventDeliveryFilteredByEventId = '';
									getEventDeliveries();
								"
							>
								<SvgComponent :width="'16'" :height="'13'" :id="'close-icon'" />
							</button>
						</div>

						<!-- coming soon abeg -->
						<!-- <button class="flex__justify-center button button__filter margin-left__24px">Batch Retry</button> -->
					</div>

					<button
						class="button button__white button__small margin-right__20px"
						@click="
							eventDeliveryFilteredByStatus = [];
							eventDeliveriesDateFilter = [];
							getEventDeliveries();
						"
						:disabled="eventDeliveryFilteredByStatus.length === 0 && eventDeliveriesDateFilter.length === 0"
					>
						Clear Filter
					</button>
				</div>
			</div>

			<div class="flex">
				<div class="dashboard--logs--table">
					<div class="table table--container has-loader" v-show="activeTab === 'events' && displayedEvents.length > 0">
						<LoaderComponent v-if="isloadingEvents" />

						<table id="events-table">
							<thead>
								<tr class="table--head">
									<th scope="col">Event Type</th>
									<th scope="col">App Name</th>
									<th scope="col">Created At</th>
									<th scope="col"></th>
								</tr>
							</thead>
							<tbody>
								<template v-for="(eventGroup, index) in displayedEvents" :key="index">
									<tr class="table--date-row">
										<td>
											<div>{{ eventGroup.date }}</div>
										</td>
										<td></td>
										<td></td>
										<td></td>
									</tr>
									<tr
										v-for="(event, index2) in eventGroup?.events"
										:key="index"
										:id="'event' + index"
										@click="
											_eventsDetailsItem = event;
											getEventDeliveriesForSidebar(event.uid);
										"
										:class="{ 'last-item': index2 === eventGroup.events.length - 1, active: event.uid === eventsDetailsItem?.uid }"
									>
										<td>
											<div>
												<div class="tag">
													{{ event?.event_type }}
												</div>
											</div>
										</td>
										<td class="has-long-text">
											<div>
												{{ event?.app_metadata.title }}
											</div>
										</td>
										<td>
											<div>
												{{ time(event?.created_at) }}
											</div>
										</td>
										<td>
											<div>
												<button
													class="button button__clear button--has-icon icon-right"
													@click="
														_eventDeliveryFilteredByEventId = event.uid;
														openDeliveriesTab();
													"
												>
													Deliveries
													<SvgComponent :width="'12'" :height="'8'" :id="'angle-arrow-right'" :styles="'margin-left__14px'" />
												</button>
											</div>
										</td>
									</tr>
								</template>
							</tbody>
						</table>

						<div class="table--load-more button--container center" v-if="events && events.pagination.totalPage > 1">
							<button
								class="button button__clear button--has-icon icon-left margin-top__20px margin-bottom__24px flex__justify-center"
								@click="
									eventsPage = eventsPage + 1;
									getEvents();
								"
								:disabled="events?.pagination?.page === events?.pagination?.totalPage || isloadingMoreEvents"
							>
								<SvgComponent v-show="!isloadingMoreEvents" :width="'24'" :height="'18'" :id="'angle-arrow-down-big'" />
								<SvgComponent v-show="isloadingMoreEvents" :width="'25'" :height="'24'" :id="'rotate-icon'" :styles="'margin-right__8px'" />
								Load more
							</button>
						</div>
					</div>

					<div class="empty-state table--container" v-show="(activeTab === 'events' && !events) || events?.content?.length === 0">
						<SvgComponent :width="'130'" :height="'110'" :id="'empty-state'" />
						<p>No event to show here</p>
					</div>

					<div class="table table--container has-loader" v-show="activeTab === 'event deliveries' && displayedEventDeliveries.length > 0">
						<LoaderComponent v-if="isloadingEventDeliveries" />

						<table id="event-deliveries-table">
							<thead>
								<tr class="table--head">
									<th scope="col">Status</th>
									<th scope="col">Event Type</th>
									<th scope="col">Attempts</th>
									<th scope="col">Created At</th>
									<th scope="col"></th>
								</tr>
							</thead>
							<tbody>
								<template v-for="(eventDeliveriesGroup, index) in displayedEventDeliveries" :key="index">
									<tr class="table--date-row">
										<td>
											<div>
												{{ eventDeliveriesGroup.date }}
											</div>
										</td>
										<td></td>
										<td></td>
										<td></td>
										<td></td>
									</tr>
									<tr
										v-for="(event, index2) in eventDeliveriesGroup.events"
										:key="index2"
										:id="'eventDels' + index2"
										:class="{ 'last-item': index2 === eventDeliveriesGroup.events.length - 1, active: event.uid === eventDelsDetailsItem?.uid }"
										@click="
											eventDelsDetailsItem = event;
											getDelieveryAttempts(event.uid);
										"
									>
										<td>
											<div class="has-retry">
												<SvgComponent v-show="event.metadata.num_trials > event.metadata.retry_limit" :width="'14'" :height="'14'" :id="'retry-icon'" />
												<div :class="'tag tag--' + event.status">
													{{ event.status }}
												</div>
											</div>
										</td>
										<td>
											<div>
												{{ event.event_metadata?.name }}
											</div>
										</td>
										<td>
											<div>
												{{ event.metadata?.num_trials }}
											</div>
										</td>
										<td>
											<div>
												{{ time(event.created_at) }}
											</div>
										</td>
										<td>
											<div>
												<button
													class="button__retry button--has-icon icon-left"
													@click="
														retryEvent({
															e: $event,
															index,
															eventDeliveryId: event.uid
														})
													"
													:disabled="event.status !== 'Failure'"
												>
													<SvgComponent :width="'14'" :height="'14'" :id="'retry-icon'" :styles="'margin-right__10px margin-top__4px'" />
													Retry
												</button>
											</div>
										</td>
									</tr>
								</template>
							</tbody>
						</table>

						<div class="table--load-more button--container center" v-if="eventDeliveries && eventDeliveries.pagination.totalPage > 1">
							<button
								class="button button__clear button--has-icon icon-left margin-top__20px margin-bottom__24px flex__justify-center"
								@click="
									eventDeliveriesPage = eventDeliveriesPage + 1;
									getEventDeliveries();
								"
								:disabled="eventDeliveries?.pagination?.page === eventDeliveries?.pagination?.totalPage || isloadingMoreEventDeliveries"
							>
								<SvgComponent v-show="!isloadingMoreEventDeliveries" :width="'24'" :height="'18'" :id="'angle-arrow-down-big'" />
								<SvgComponent v-show="isloadingMoreEventDeliveries" :width="'25'" :height="'24'" :id="'rotate-icon'" :styles="'margin-right__8px'" />
								Load more
							</button>
						</div>
					</div>

					<div class="empty-state table--container" v-show="(activeTab === 'event deliveries' && !eventDeliveries) || eventDeliveries?.content?.length === 0">
						<SvgComponent :width="'130'" :height="'110'" :id="'empty-state'" />
						<p>No event to show here</p>
					</div>
				</div>

				<div class="dashboard--logs--details has-loader">
					<template v-if="activeTab === 'event deliveries'">
						<LoaderComponent v-if="isloadingEventDeliveries || isloadingDeliveryAttempts" />

						<h3>Details</h3>
						<ul class="dashboard--logs--details--meta">
							<li class="list-item-inline">
								<div class="list-item-inline--label">IP Address</div>
								<div class="list-item-inline--item color">{{ eventDeliveryAtempt?.ip_address || '-' }}</div>
							</li>
							<li class="list-item-inline">
								<div class="list-item-inline--label">HTTP Status</div>
								<div class="list-item-inline--item">{{ eventDeliveryAtempt?.http_status || '-' }}</div>
							</li>
							<li class="list-item-inline">
								<div class="list-item-inline--label">API Version</div>
								<div class="list-item-inline--item color">{{ eventDeliveryAtempt?.api_version || '-' }}</div>
							</li>
							<li class="list-item-inline">
								<div class="list-item-inline--label">Endpoint</div>
								<div class="list-item-inline--item color">
									{{ eventDelsDetailsItem?.endpoint?.target_url }}
								</div>
							</li>
							<li class="list-item-inline" v-if="eventDelsDetailsItem?.metadata.num_trials < eventDelsDetailsItem?.metadata.retry_limit && eventDelsDetailsItem?.status !== 'Success'">
								<div class="list-item-inline--label">Next Retry</div>
								<div class="list-item-inline--item color">{{ date(eventDelsDetailsItem?.metadata?.next_send_time) }} - {{ time(eventDelsDetailsItem.metadata?.next_send_time) }}</div>
							</li>
							<li class="list-item-inline">
								<div class="list-item-inline--label">App Name</div>
								<div class="list-item-inline--item color">
									{{ eventDelsDetailsItem?.app_metadata?.title }}
								</div>
							</li>
							<li class="list-item-inline" v-if="eventDelsDetailsItem?.status === 'Success'">
								<div class="list-item-inline--label">Delivery Time</div>
								<div class="list-item-inline--item color">{{ date(eventDelsDetailsItem?.updated_at) }} - {{ time(eventDelsDetailsItem?.updated_at) }}</div>
							</li>
						</ul>

						<ul class="tabs tabs__logs">
							<li v-for="(tab, index) in eventDetailsTabs" :key="'tabEventDels' + index" :class="eventDetailsActiveTab === tab.id ? 'active' : ''">
								<button @click="eventDetailsActiveTab = tab.id">{{ tab.label }}</button>
							</li>
						</ul>

						<div class="dashboard--logs--details--req-res has-loader">
							<!-- <convoy-loader *ngIf="(eventDetailsActiveTab === 'response' || eventDetailsActiveTab === 'request') && isloadingDeliveryAttempts"></convoy-loader> -->

							<div :class="'dashboard--logs--details--tabs-data ' + (eventDetailsActiveTab === 'data' ? 'show' : '')">
								<h3>Event</h3>
								<pre
									class="line-numbers lang-javascript"
								><code class="language-javascript">{{eventDelsDetailsItem?.metadata?.data ? JSON.stringify(eventDelsDetailsItem?.metadata.data, null, 4)?.replaceAll(/"([^"]+)":/g, '$1:') : 'No event sent'}}</code></pre>
							</div>

							<div :class="'dashboard--logs--details--tabs-data ' + (eventDetailsActiveTab === 'response' && !isloadingDeliveryAttempts ? 'show' : '')">
								<template v-if="!eventDeliveryAtempt?.error">
									<h3>Header</h3>
									<pre
										class="line-numbers lang-javascript"
									><code class="language-javascript">{{JSON.stringify(eventDeliveryAtempt.response_http_header, null, 4)?.replaceAll(/"([^"]+)":/g, '$1:')}}</code></pre>

									<h3>Body</h3>
									<pre
										class="line-numbers lang-javascript"
									><code class="language-javascript">{{eventDeliveryAtempt?.response_data ? eventDeliveryAtempt.response_data : 'No response body was sent'}}</code></pre>
								</template>

								<template v-if="eventDeliveryAtempt?.error">
									<h3>Error</h3>
									<pre
										class="line-numbers lang-javascript"
									><code class="language-javascript">{{JSON.stringify(this.eventDeliveryAtempt.error, null, 4)?.replaceAll(/"([^"]+)":/g, '$1:')}}</code></pre>
								</template>
							</div>

							<div :class="'dashboard--logs--details--tabs-data ' + (eventDetailsActiveTab === 'request' && !isloadingDeliveryAttempts ? 'show' : '')">
								<h3>Header</h3>
								<pre class="line-numbers lang-javascript"><code class="language-javascript">{{eventDeliveryAtempt?.request_http_header
                                    ? JSON.stringify(eventDeliveryAtempt.request_http_header, null, 4)?.replaceAll(/"([^"]+)":/g, '$1:')
                                    : 'No request header was sent'}}</code></pre>
							</div>
						</div>
					</template>

					<template v-if="activeTab === 'events'">
						<LoaderComponent v-if="isloadingEvents || isloadingEventDeliveries" />

						<h3>Details</h3>
						<div class="dashboard--logs--details--req-res">
							<div class="dashboard--logs--details--tabs-data show">
								<h3>Event</h3>
								<pre
									class="line-numbers lang-javascript"
								><code class="language-javascript">{{eventsDetailsItem?.data ? JSON.stringify(eventsDetailsItem.data, null, 4)?.trim().replaceAll(/"([^"]+)":/g, '$1:') : 'No event to display'}}</code></pre>
							</div>
						</div>

						<h4>Deliveries Overview</h4>
						<ul class="dashboard--logs--details--endpoints inline">
							<li v-for="(delivery, index) in sidebarEventDeliveries">
								<div :class="'tag tag--' + delivery.status">{{ delivery.status }}</div>
								<div class="url">
									{{ delivery.endpoint.target_url }}
								</div>
							</li>
							<li v-if="sidebarEventDeliveries.length === 0">
								<p>No event delivery sent for this event</p>
							</li>
						</ul>
					</template>
				</div>
			</div>
		</section>
	</div>
</template>

<style>
@import '../../node_modules/convoy-ui/css/main.css';
@import '../styles/prism.css';

.mx-datepicker.button__filter {
	width: fit-content;
	padding-left: 40px;
}

.mx-datepicker.button__filter input {
	max-width: unset;
	width: 181px;
}

.mx-datepicker .mx-input {
	height: unset;
	padding: 0;
	padding-left: 0;
	font-size: inherit;
	line-height: initial;
	color: inherit;
	box-shadow: none;
}

.mx-datepicker .mx-icon-calendar,
.mx-datepicker .mx-icon-clear {
	left: -24px;
	right: unset;
}

.mx-datepicker svg {
	width: 13px;
}

.empty-table {
	text-align: center;
	height: 50px;
	margin-top: 25px;
	font-style: italic;
	font-size: 12px;
}
</style>
