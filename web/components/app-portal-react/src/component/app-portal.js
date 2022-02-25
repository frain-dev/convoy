import './style.scss';
import React, { useEffect, useState, useCallback } from 'react';
import ArrowDownIcon from '../assets/img/arrow-down-icon.svg';
import CloseIcon from '../assets/img/close-icon.svg';
import RefreshIcon from '../assets/img/refresh-icon.svg';
// import RefreshIcon2 from '../assets/img/refresh-icon-2.svg';
import CalendarIcon from '../assets/img/calendar-icon.svg';
import AngleArrowDownIcon from '../assets/img/angle-arrow-down.svg';
import StatusFilterIcon from '../assets/img/status-filter-icon.svg';
import AngleArrowRightIcon from '../assets/img/angle-arrow-right-primary.svg';
import RetryIcon from '../assets/img/retry-icon.svg';
import EmptyStateImage from '../assets/img/empty-state-img.svg';
import { DateRange } from 'react-date-range';
import { request } from '../services/https.service';
import './style.scss';
import 'react-date-range/dist/styles.css';
import 'react-date-range/dist/theme/default.css';
import { showNotification } from '../components/app-notification';
import { getDate, getTime } from '../helpers/common.helper';
import Prism from 'prismjs';
import '../scss/prism.scss';
import '../helpers/prism-line-plugin';

const moment = require('moment');

function AppPortal() {
	const [eventDeliveryEventId, setEventDeliveryEventId] = useState('');
	const [eventDeliveryStatuses] = useState(['Success', 'Failure', 'Retry', 'Scheduled', 'Processing', 'Discarded']);
	const [events, setEventsData] = useState({ content: [], pagination: { page: 1, totalPage: 0 } });
	const [eventDeliveries, setEventDeliveriesData] = useState({ content: [], pagination: { page: 1, totalPage: 0 } });
	const [eventDeliveriesSidebar, setEventDeliveriesDataSidebar] = useState([]);
	const [eventDelFilterStatus, setEventDelFilterStatus] = useState([]);
	const [displayedEvents, setDisplayedEvents] = useState([]);
	const [displayedEventDeliveries, setDisplayedEventDeliveries] = useState([]);
	const [tabs] = useState(['events', 'event deliveries']);
	const [activeTab, setActiveTab] = useState('events');
	const [appDetails, setAppDetails] = useState();
	const [showEventDeliveriesStatusDropdown, toggleShowEventDeliveriesStatusDropdown] = useState();
	const [eventDetailsTabs] = useState([
		{ id: 'data', label: 'Event' },
		{ id: 'response', label: 'Response' },
		{ id: 'request', label: 'Request' }
	]);
	const [eventDetailsActiveTab, setEventDetailsActiveTab] = useState('data');
	const [eventDateFilterActive, toggleEventDateFilterActive] = useState(false);
	const [eventDelDateFilterActive, toggleEventDelDateFilterActive] = useState(false);
	const [showEventFilterCalendar, toggleShowEventFilterCalendar] = useState(false);
	const [showEventDelFilterCalendar, toggleEventDelFilterCalendar] = useState(false);
	const [eventDelStatusFilterActive, toggleEventDelStatusFilterActive] = useState(false);
	const [eventDeliveryAtempt, setEventDeliveryAtempt] = useState({
		ip_address: '',
		http_status: '',
		api_version: '',
		updated_at: 0,
		deleted_at: 0,
		response_data: '',
		response_http_header: {},
		request_http_header: {}
	});
	const [eventDetailsItem, setEventDetailsItem] = useState();
	const [eventDelDetailsItem, setEventDelDetailsItem] = useState();
	const [eventFilterDates, setEventFilterDates] = useState([
		{
			startDate: new Date(),
			endDate: new Date(),
			key: 'selection'
		}
	]);
	const [eventDelFilterDates, setEventDelFilterDates] = useState([
		{
			startDate: new Date(),
			endDate: new Date(),
			key: 'selection'
		}
	]);

	const setEventsDisplayed = events => {
		const dateCreateds = events.map(event => getDate(event.created_at));
		const uniqueDateCreateds = [...new Set(dateCreateds)];
		const displayedEvents = [];
		uniqueDateCreateds.forEach(eventDate => {
			const filteredEventDate = events.filter(event => getDate(event.created_at) === eventDate);
			const eventsItem = { date: eventDate, events: filteredEventDate };
			displayedEvents.push(eventsItem);
		});
		setDisplayedEvents(displayedEvents);
	};

	const setEventDeliveriesDisplayed = eventDels => {
		const dateCreateds = eventDels.map(event => getDate(event.created_at));
		const uniqueDateCreateds = [...new Set(dateCreateds)];
		const displayedEvents = [];
		uniqueDateCreateds.forEach(eventDate => {
			const filteredEventDate = eventDels.filter(event => getDate(event.created_at) === eventDate);
			const eventsItem = { date: eventDate, events: filteredEventDate };
			displayedEvents.push(eventsItem);
		});
		setDisplayedEventDeliveries(displayedEvents);
	};

	const setDateForFilter = ({ startDate, endDate }) => {
		if (!endDate && !startDate) return { startDate: '', endDate: '' };
		startDate = String(moment(`${moment(startDate).format('YYYY[-]MM[-]DD')} 00:00:00`).toISOString(true)).split('.')[0];
		endDate = String(moment(`${moment(endDate).format('YYYY[-]MM[-]DD')} 23:59:59`).toISOString(true)).split('.')[0];
		return { startDate, endDate };
	};

	const clearEventDeliveriesDateFilter = () => {
		setEventDelFilterDates([
			{
				startDate: new Date(),
				endDate: new Date(),
				key: 'selection'
			}
		]);
		toggleEventDelDateFilterActive(false);
		getEventDeliveries({ page: 1 });
	};

	const clearEventDeliveriesFilters = () => {
		setEventDelFilterDates([
			{
				startDate: new Date(),
				endDate: new Date(),
				key: 'selection'
			}
		]);
		setEventDeliveryEventId('');
		document.querySelectorAll('.dropdown--list--item input[type=checkbox]').forEach(el => (el.checked = false));
		setEventDelFilterStatus([]);
		toggleEventDelDateFilterActive(false);
		getEventDeliveries({ page: 1, eventDelFilterStatusList: [] });
	};

	const clearEventsFilters = () => {
		setEventFilterDates([
			{
				startDate: new Date(),
				endDate: new Date(),
				key: 'selection'
			}
		]);
		toggleEventDateFilterActive(false);
		getEvents({ page: 1 });
	};

	const getEvents = useCallback(async ({ page, eventsData, dates }) => {
		toggleShowEventFilterCalendar(false);

		if (!dates) dates = [{ startDate: null, endDate: null }];

		const dateFromPicker = dates[0];
		const { startDate, endDate } = setDateForFilter(dateFromPicker);

		try {
			const eventsResponse = await (
				await request({
					url: `/events?sort=AESC&page=${page || 1}&perPage=20&startDate=${startDate}&endDate=${endDate}&appId=${'291e98cb-4e93-408f-bb5b-d422ff13d12c'}`,
					method: 'GET'
				})
			).data;

			if (eventsData && eventsData?.pagination?.next === page) {
				const content = [...eventsData.content, ...eventsResponse.data.content];
				const pagination = eventsResponse.data.pagination;
				setEventsData({ content, pagination });
				setEventsDisplayed(content);
				return;
			}

			setEventsData(eventsResponse.data);
			setEventsDisplayed(eventsResponse.data.content);
			setEventDetailsItem(eventsResponse.data.content[0]);
			eventDeliveriesForSidebar({ eventId: eventsResponse.data.content[0].uid });
			Prism.highlightAll();
		} catch (error) {
			return error;
		}
	}, []);

	const getEventDeliveries = useCallback(async ({ page, eventsData, dates, eventId, eventDelFilterStatusList = [] }) => {
		let eventDeliveryStatusFilterString = '';
		toggleEventDelFilterCalendar(false);
		toggleEventDelStatusFilterActive(eventDelFilterStatusList.length > 0);
		eventDelFilterStatusList.forEach(status => (eventDeliveryStatusFilterString += `&status=${status}`));

		if (!dates) dates = [{ startDate: null, endDate: null }];

		const dateFromPicker = dates[0];
		const { startDate, endDate } = setDateForFilter(dateFromPicker);

		try {
			const eventsResponse = await (
				await request({
					url: `/eventdeliveries?sort=AESC&page=${page || 1}&perPage=20&startDate=${startDate}&endDate=${endDate}&appId=${'291e98cb-4e93-408f-bb5b-d422ff13d12c'}&eventId=${
						eventId || eventDeliveryEventId || ''
					}${eventDeliveryStatusFilterString || ''}`,
					method: 'GET'
				})
			).data;

			if (eventsData && eventsData?.pagination?.next === page) {
				const content = [...eventsData.content, ...eventsResponse.data.content];
				const pagination = eventsResponse.data.pagination;
				setEventDeliveriesData({ content, pagination });
				setEventDeliveriesDisplayed(content);
				return;
			}

			setEventDeliveriesData(eventsResponse.data);
			setEventDeliveriesDisplayed(eventsResponse.data.content);
			setEventDelDetailsItem(eventsResponse.data.content[0]);
			getDelieveryAttempts(eventsResponse.data.content[0].uid);
			Prism.highlightAll();
		} catch (error) {
			return error;
		}
	}, []);

	const eventDeliveriesForSidebar = useCallback(async ({ eventId }) => {
		try {
			const eventsResponse = await (
				await request({
					url: `/eventdeliveries?eventId=${eventId}`,
					method: 'GET'
				})
			).data;
			setEventDeliveriesDataSidebar(eventsResponse.data.content);
		} catch (error) {
			return error;
		}
	}, []);

	const getAppDetails = useCallback(async () => {
		try {
			const appDetailsResponse = await (
				await request({
					url: `/apps/291e98cb-4e93-408f-bb5b-d422ff13d12c/?groupID=${'5c9c6db0-7606-4f9f-9965-5455980881a2'}`
				})
			).data;
			setAppDetails(appDetailsResponse.data);
		} catch (error) {
			return error;
		}
	}, []);

	const getDelieveryAttempts = async eventId => {
		try {
			const deliveryAttemptsResponse = await (
				await request({
					url: `/eventdeliveries/${eventId}/deliveryattempts`
				})
			).data;
			setEventDeliveryAtempt(deliveryAttemptsResponse.data[deliveryAttemptsResponse.data.length - 1]);
			Prism.highlightAll();
		} catch (error) {
			return error;
		}
	};

	const retryEvent = async ({ eventId, e, index }) => {
		e.stopPropagation();
		const retryButton = document.querySelector(`#eventDel${index} button`);
		retryButton.classList.add(['spin', 'disable_action']);
		retryButton.disabled = true;

		try {
			await (
				await request({
					method: 'PUT',
					url: `/eventdeliveries/${eventId}/resend`
				})
			).data;
			showNotification({ message: 'Retry Request Sent' });
			retryButton.classList.remove(['spin', 'disable_action']);
			retryButton.disabled = false;
			getEvents({ page: events.pagination.page });
		} catch (error) {
			showNotification({ message: error.response.data.message });
			retryButton.classList.remove(['spin', 'disable_action']);
			retryButton.disabled = false;
			return error;
		}
	};

	const updateEventDevliveryStatusFilter = ({ status, input }) => {
		if (input.target.checked) {
			eventDelFilterStatus.push(status);
			setEventDelFilterStatus(eventDelFilterStatus);
		} else {
			let index = eventDelFilterStatus.findIndex(x => x === status);
			eventDelFilterStatus.splice(index, 1);
			setEventDelFilterStatus(eventDelFilterStatus);
		}
	};

	const toggleActiveTab = tab => {
		setActiveTab(tab);
	};

	useEffect(() => {
		// const getAppDetails = async () => {
		// 	try {
		// 		const appDetailsResponse = await this.convyAppService.request({
		// 			url: this.getAPIURL(`/apps/7ea6d36a-f988-4623-abeb-2ad2dd3d2c7b?groupID=${this.activeGroup || ''}`),
		// 			method: 'get'
		// 		});
		// 		setAppDetails(appDetailsResponse.data);
		// 		// this.appDetails = appDetailsResponse.data;
		// 	} catch (error) {
		// 		return error;
		// 	}
		// };

		// const getOrganisationDetails = async () => {
		// 	try {
		// 		const organisationDetailsResponse = await (
		// 			await request({
		// 				url: `/dashboard/config`
		// 			})
		// 		).data;
		// 		setOrganisationDetails(organisationDetailsResponse.data);
		// 	} catch (error) {
		// 		return error;
		// 	}
		// };

		// const fetchDashboardData = async () => {
		// 	try {
		// 		const { startDate, endDate } = setDateForFilter(filterDates[0]);
		// 		const dashboardResponse = await request({
		// 			url: `/dashboard/summary?startDate=${startDate}&endDate=${endDate}&type=${filterFrequency || 'daily'}`
		// 		});
		// 		setDashboardData(dashboardResponse.data.data);
		// 	} catch (error) {
		// 		return error;
		// 	}
		// };

		// fetchDashboardData();
		// getOrganisationDetails();
		// getAppDetails();
		getAppDetails();
		getEvents({ page: 1 });
		getEventDeliveries({ page: 1 });
	}, [getEvents, getAppDetails, getEventDeliveries]);

	return (
		<div className="app-page">
			<div className="app-page--head">
				<h3>Endpoint</h3>
			</div>

			<div className="app-page--details">
				<div className="card app-page--endpoints">
					<table>
						<thead>
							<tr className="table--head">
								<th scope="col">Endpoint URL</th>
								<th scope="col">Created At</th>
								<th scope="col">Updated At</th>
								<th scope="col">Endpoint Events</th>
								<th scope="col">Status</th>
							</tr>
						</thead>
						<tbody>
							{appDetails?.endpoints.map((endpoint, index) => (
								<tr className="has-border" key={index}>
									<td className="has-long-text longer">
										<div title={endpoint.target_url}>{endpoint.target_url}</div>
									</td>
									<td>
										<div>{getDate(endpoint.created_at)}</div>
									</td>
									<td>
										<div>{getDate(endpoint.updated_at)}</div>
									</td>
									<td>
										<div>
											{endpoint.events.map((event, idx) => (
												<div className="tag" key={idx}>
													{event}
												</div>
											))}
										</div>
									</td>
									<td>
										<div>
											<div className={'tag tag--' + (endpoint.status === 'active' ? 'Success' : 'Retry')}>{endpoint.status}</div>
										</div>
									</td>
								</tr>
							))}
						</tbody>
					</table>
				</div>
			</div>

			<section className="card dashboard--logs">
				<div className="dashboard--logs--tabs">
					<div className="dashboard--logs--tabs--head tabs">
						<div className="tabs">
							{tabs.map((tab, index) => (
								<button onClick={() => toggleActiveTab(tab)} key={index} className={'clear tab ' + (activeTab === tab ? 'active' : '')}>
									{tab}
								</button>
							))}
						</div>

						{activeTab === 'events' && <div className="filter"></div>}
					</div>

					<div className="table">
						<div className={displayedEvents.length > 0 && activeTab && activeTab === 'events' ? '' : 'hidden'}>
							<div className="filter">
								<button className={'filter--button ' + (eventDateFilterActive ? 'active' : '')} onClick={() => toggleShowEventFilterCalendar(!showEventFilterCalendar)}>
									<img src={CalendarIcon} alt="calender icon" />
									<div>Date</div>
									<img src={AngleArrowDownIcon} alt="arrow down icon" />
								</button>
								{showEventFilterCalendar && (
									<div className="date-filter--container">
										<DateRange onChange={item => setEventFilterDates([item.selection])} editableDateInputs={true} moveRangeOnFirstSelection={false} ranges={eventFilterDates} />
										<div className="button-container">
											<button
												className="primary"
												onClick={() => {
													getEvents({ dates: eventFilterDates });
													toggleEventDateFilterActive(true);
												}}>
												Apply
											</button>
											<button
												className="primary outline"
												onClick={() => {
													getEvents({ page: 1 });
													toggleEventDateFilterActive(false);
												}}>
												Clear
											</button>
										</div>
									</div>
								)}

								<button className={'filter--button primary events-filter-clear-btn' + (!eventDateFilterActive ? ' disabled' : '')} onClick={() => clearEventsFilters()}>
									Clear Filter
								</button>
							</div>

							<hr />

							<div className="table--container smaller-table">
								<table id="events-table">
									<thead>
										<tr className="table--head">
											<th scope="col">Event Type</th>
											<th scope="col">App Name</th>
											<th scope="col">Created At</th>
											<th scope="col"></th>
										</tr>
									</thead>
									<tbody>
										{displayedEvents.map((eventGroup, index) => (
											<React.Fragment key={'eventGroup' + index}>
												<tr className="table--date-row">
													<td>
														<div>{eventGroup.date}</div>
													</td>
													<td></td>
													<td></td>
													<td></td>
													<td></td>
													<td></td>
												</tr>
												{eventGroup.events.map((event, index) => (
													<tr
														key={'events' + index}
														onClick={() => {
															eventDeliveriesForSidebar({ eventId: event.uid });
															setEventDetailsItem(event);
															Prism.highlightAll();
														}}
														className={event.uid === eventDetailsItem?.uid ? 'active' : ''}
														id={'event' + index}>
														<td>
															<div>
																<div className="tag">{event.event_type}</div>
															</div>
														</td>

														<td className="has-long-text">
															<div>{event.app_metadata.title}</div>
														</td>
														<td>
															<div>{getTime(event.created_at)}</div>
														</td>
														<td>
															<div>
																<button
																	className="primary clear has-icon icon-right"
																	onClick={e => {
																		e.stopPropagation();
																		setEventDeliveryEventId(event.uid);
																		getEventDeliveries({ page: 1, eventId: event.uid });
																		toggleActiveTab('event deliveries');
																	}}>
																	Deliveries
																	<img src={AngleArrowRightIcon} alt="arrow right" />
																</button>
															</div>
														</td>

														{/*<td>
																<div>{event.event_type}</div>
															</td>
															<td>
																<div>{event.metadata?.num_trials}</div>
															</td>
															<td>
																<div>{event.metadata?.num_trials < event.metadata?.retry_limit && event.status !== 'Success' ? getTime(event.metadata.next_send_time) : '-'}</div>
															</td>
															<td>
																<div>{getTime(event.created_at)}</div>
															</td>
															<td>
																<div>
																	<button
																		disabled={event.status === 'Success' || event.status === 'Scheduled'}
																		className={'primary has-icon icon-left ' + (event.status === 'Success' || event.status === 'Scheduled' ? 'disable_action' : '')}
																		onClick={e => retryEvent({ eventId: event.uid, appId: event.app_id, e, index })}>
																		<img src={RefreshIcon} alt="refresh icon" />
																		Retry
																	</button>
																</div>
                                                            </td>*/}
													</tr>
												))}
											</React.Fragment>
										))}
									</tbody>
								</table>

								{events.pagination.totalPage > 1 && (
									<div className=" table--load-more button-container margin-top center">
										<button
											className={'primary clear has-icon icon-left ' + (events.pagination.page === events.pagination.totalPage ? 'disable_action' : '')}
											disabled={events.pagination.page === events.pagination.totalPage}
											onClick={() => getEvents({ page: events.pagination.page + 1, eventsData: events, dates: eventDateFilterActive ? eventFilterDates : null })}>
											<img src={ArrowDownIcon} alt="arrow down icon" />
											Load more
										</button>
									</div>
								)}
							</div>
						</div>

						<div className={displayedEventDeliveries.length > 0 && activeTab && activeTab === 'event deliveries' ? '' : 'hidden'}>
							<div className="filter">
								<button className={'filter--button ' + (eventDelDateFilterActive ? 'active' : '')} onClick={() => toggleEventDelFilterCalendar(!showEventDelFilterCalendar)}>
									<img src={CalendarIcon} alt="calender icon" />
									<div>Date</div>
									<img src={AngleArrowDownIcon} alt="arrow down icon" />
								</button>
								{showEventDelFilterCalendar && (
									<div className="date-filter--container">
										<DateRange onChange={item => setEventDelFilterDates([item.selection])} editableDateInputs={true} moveRangeOnFirstSelection={false} ranges={eventDelFilterDates} />
										<div className="button-container">
											<button
												className="primary"
												onClick={() => {
													getEventDeliveries({ dates: eventFilterDates });
													toggleEventDelDateFilterActive(true);
												}}>
												Apply
											</button>
											<button
												className="primary outline"
												onClick={() => {
													clearEventDeliveriesDateFilter();
													toggleEventDelDateFilterActive(false);
												}}>
												Clear
											</button>
										</div>
									</div>
								)}

								<div className="dropdown">
									<button
										className={'filter--button dropdown--button' + (eventDelStatusFilterActive ? ' active' : '')}
										onClick={() => toggleShowEventDeliveriesStatusDropdown(!showEventDeliveriesStatusDropdown)}>
										<img src={StatusFilterIcon} alt="status filter icon" />
										<span>Status</span>
										<img src={AngleArrowDownIcon} alt="arrow down icon" />
									</button>
									<div className={'dropdown--list' + (showEventDeliveriesStatusDropdown ? ' show' : '')}>
										{eventDeliveryStatuses.map((status, index) => (
											<div className="dropdown--list--item" key={'status' + index}>
												<input type="checkbox" name={status} value={status} id={status} onChange={e => updateEventDevliveryStatusFilter({ status, input: e })} />
												<label htmlFor={status}>{status}</label>
											</div>
										))}
										<button className="primary" onClick={() => getEventDeliveries({ page: 1, eventDelFilterStatusList: eventDelFilterStatus })}>
											Apply
										</button>
									</div>
								</div>

								{eventDeliveryEventId && (
									<div className="filter--button event-button active">
										Event Filtered
										<button
											className="primary clear has-icon"
											onClick={() => {
												setEventDeliveryEventId('');
												getEventDeliveries({ page: 1 });
											}}>
											<img src={CloseIcon} alt="close icon" />
										</button>
									</div>
								)}

								<button
									className={'filter--button primary events-filter-clear-btn' + (!eventDelDateFilterActive && eventDeliveryEventId === '' && !eventDelStatusFilterActive ? ' disabled' : '')}
									onClick={() => clearEventDeliveriesFilters()}>
									Clear Filter
								</button>
							</div>

							<hr />

							{/*<div className="table--actions button-container left">
								<button className="primary clear has-icon icon-left hover">
									<img src={RefreshIcon2} alt="refresh icon" />
									Refresh
								</button>
								<button className="primary clear has-icon icon-left hover">
									<img src={RetryIcon} alt="retry icon" />
									Bulk Retry
								</button>
                            </div>*/}

							<div className="table--container">
								<table id="events-deliveries-table">
									<thead>
										<tr className="table--head">
											<th scope="col" className="checkbox">
												<div className="checkbox">
													<input type="checkbox" name="eventDeliveryTable" id="eventDeliveryTable" />
												</div>
												Status
											</th>
											<th scope="col">Event Type</th>
											<th scope="col">Attempts</th>
											<th scope="col">Created At</th>
											<th scope="col"></th>
										</tr>
									</thead>
									<tbody>
										{displayedEventDeliveries.map((eventDeliveriesGroup, index) => (
											<React.Fragment key={'eventDelsGroup' + index}>
												<tr className="table--date-row">
													<td>
														<div>{eventDeliveriesGroup.date}</div>
													</td>
													<td></td>
													<td></td>
													<td></td>
													<td></td>
												</tr>
												{eventDeliveriesGroup.events.map((event, index) => (
													<tr
														className={event.uid === eventDelDetailsItem?.uid ? 'active' : ''}
														id={'eventDel' + index}
														key={'eventDels' + index}
														onClick={() => {
															Prism.highlightAll();
															setEventDelDetailsItem(event);
															getDelieveryAttempts(event.uid);
														}}>
														<td>
															<div className="checkbox has-retry">
																{event.metadata?.num_trials > event.metadata?.retry_limit && <img src={RetryIcon} alt="retry icon" title="manual retried" />}
																<input type="checkbox" id="event" />
																<div className={'tag tag--' + event.status}>{event.status}</div>
															</div>
														</td>
														<td>
															<div>{event.event_metadata?.name}</div>
														</td>
														<td>
															<div>{event.metadata?.num_trials}</div>
														</td>
														<td>
															<div>{getTime(event.created_at)}</div>
														</td>
														<td>
															<div>
																<button
																	className={'primary has-icon icon-left' + (event.uid === eventDetailsItem?.uid ? ' active' : '')}
																	onClick={e => retryEvent({ eventId: event.uid, e, index })}>
																	<img src={RefreshIcon} alt="refresh icon" />
																	Retry
																</button>
															</div>
														</td>
													</tr>
												))}
											</React.Fragment>
										))}
									</tbody>
								</table>

								{eventDeliveries.pagination.totalPage > 1 && (
									<div className=" table--load-more button-container margin-top center">
										<button
											className={'primary clear has-icon icon-left ' + (eventDeliveries.pagination.page === eventDeliveries.pagination.totalPage ? 'disable_action' : '')}
											disabled={eventDeliveries.pagination.page === eventDeliveries.pagination.totalPage}
											onClick={() =>
												getEventDeliveries({ page: eventDeliveries.pagination.page + 1, eventsData: eventDeliveries, dates: eventDelDateFilterActive ? eventDelFilterDates : null })
											}>
											<img src={ArrowDownIcon} alt="arrow down icon" />
											Load more
										</button>
									</div>
								)}
							</div>
						</div>

						{activeTab === 'events' && displayedEvents.length === 0 && (
							<div className="empty-state">
								<img src={EmptyStateImage} alt="empty state" />
								<p>No {activeTab} to show here</p>
							</div>
						)}
					</div>
				</div>

				<div className="dashboard--logs--details">
					<div className={eventDetailsItem && activeTab === 'events' ? '' : 'hidden'}>
						<h3>Details</h3>

						<div className="dashboard--logs--details--req-res">
							<div className={'dashboard--logs--details--tabs-data ' + (eventDetailsActiveTab === 'data' ? 'show' : '')}>
								<h3>Event</h3>
								<pre className="line-numbers">
									<code className="lang-javascript">{eventDetailsItem?.data ? JSON.stringify(eventDetailsItem.data, null, 4).replaceAll(/"([^"]+)":/g, '$1:') : ''}</code>
								</pre>
							</div>
						</div>

						<h4>Deliveries Overview</h4>
						<ul className="dashboard--logs--details--endpoints inline">
							{eventDeliveriesSidebar.length > 0 &&
								eventDeliveriesSidebar.map((event, index) => (
									<li key={'endpoint' + index}>
										<div className={'tag tag--' + event.status}>{event.status}</div>
										<div className="url" title="delivery.endpoint.target_url">
											{event.endpoint.target_url}
										</div>
									</li>
								))}
						</ul>
					</div>

					<div className={eventDelDetailsItem && activeTab === 'event deliveries' ? '' : 'hidden'}>
						<h3>Details</h3>
						<ul className="dashboard--logs--details--meta">
							<li className="list-item-inline">
								<div className="list-item-inline--label">IP Address</div>
								<div className="list-item-inline--item color">{eventDeliveryAtempt?.ip_address || '-'}</div>
							</li>
							<li className="list-item-inline">
								<div className="list-item-inline--label">HTTP Status</div>
								<div className="list-item-inline--item">{eventDeliveryAtempt?.http_status || '-'}</div>
							</li>
							<li className="list-item-inline">
								<div className="list-item-inline--label">API Version</div>
								<div className="list-item-inline--item color">{eventDeliveryAtempt?.api_version || '-'}</div>
							</li>
							<li className="list-item-inline">
								<div className="list-item-inline--label">Endpoint</div>
								<div className="list-item-inline--item color">{eventDelDetailsItem?.endpoint?.target_url}</div>
							</li>
							<li className="list-item-inline">
								<div className="list-item-inline--label">Next Retry</div>
								<div className="list-item-inline--item color">{eventDelDetailsItem?.metadata.next_send_time}</div>
							</li>
							<li className="list-item-inline">
								<div className="list-item-inline--label">App Name</div>
								<div className="list-item-inline--item color">{eventDelDetailsItem?.app_metadata.title}</div>
							</li>
							<li className="list-item-inline">
								<div className="list-item-inline--label">Delivery Time</div>
								<div className="list-item-inline--item color">{eventDelDetailsItem?.updated_at}</div>
							</li>
						</ul>

						<ul className="tabs">
							{eventDetailsTabs.map(tab => (
								<li className={'tab ' + (eventDetailsActiveTab === tab.id ? 'active' : '')} key={tab.id}>
									<button className="primary outline" onClick={() => setEventDetailsActiveTab(tab.id)}>
										{tab.label}
									</button>
								</li>
							))}
						</ul>

						<div className="dashboard--logs--details--req-res">
							<div className={'dashboard--logs--details--tabs-data ' + (eventDetailsActiveTab === 'data' ? 'show' : '')}>
								<pre className="line-numbers">
									<code className="lang-javascript">
										{eventDelDetailsItem?.metadata.data ? JSON.stringify(eventDelDetailsItem.metadata.data, null, 4).replaceAll(/"([^"]+)":/g, '$1:') : ''}
									</code>
								</pre>
							</div>

							<div className={'dashboard--logs--details--tabs-data ' + (eventDetailsActiveTab === 'response' ? 'show' : '')}>
								<h3>Header</h3>
								<pre className="line-numbers">
									<code className="lang-javascript">
										{eventDeliveryAtempt?.response_http_header
											? JSON.stringify(eventDeliveryAtempt.response_http_header, null, 4).replaceAll(/"([^"]+)":/g, '$1:')
											: 'No response header was sent'}
									</code>
								</pre>

								<h3>Body</h3>
								<pre className="line-numbers">
									<code className="lang-html">{eventDeliveryAtempt?.response_data ? eventDeliveryAtempt.response_data : 'No response body was sent'}</code>
								</pre>
							</div>

							<div className={'dashboard--logs--details--tabs-data ' + (eventDetailsActiveTab === 'request' ? 'show' : '')}>
								<h3>Header</h3>
								<pre className="line-numbers">
									<code className="lang-javascript">
										{eventDeliveryAtempt?.request_http_header
											? JSON.stringify(eventDeliveryAtempt.request_http_header, null, 4).replaceAll(/"([^"]+)":/g, '$1:')
											: 'No request header was sent'}
									</code>
								</pre>
							</div>
						</div>
					</div>
				</div>
			</section>
		</div>
	);
}

export default AppPortal;
