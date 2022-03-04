import React, { useEffect, useState, useCallback } from 'react';
import ArrowDownIcon from 'assets/img/arrow-down-icon.svg';
import CloseIcon from 'assets/img/close-icon.svg';
import RefreshIcon from 'assets/img/refresh-icon.svg';
// import RefreshIcon2 from 'assets/img/refresh-icon-2.svg';
import CalendarIcon from 'assets/img/calendar-icon.svg';
import AngleArrowDownIcon from 'assets/img/angle-arrow-down.svg';
import StatusFilterIcon from 'assets/img/status-filter-icon.svg';
import AngleArrowRightIcon from 'assets/img/angle-arrow-right-primary.svg';
import RetryIcon from 'assets/img/retry-icon.svg';
import EmptyStateImage from 'assets/img/empty-state-img.svg';

import { DateRange } from 'react-date-range';
import styles from './styles.scss';
import 'react-date-range/dist/styles.css';
import 'react-date-range/dist/theme/default.css';
import Prism from 'prismjs';

const moment = require('moment');

import * as axios from 'axios';
const _axios = axios.default;

const months = ['Jan', 'Feb', 'Mar', 'April', 'May', 'June', 'July', 'Aug', 'Sept', 'Oct', 'Nov', 'Dec'];

const getDate = date => {
	const _date = new Date(date);
	const day = _date.getDate();
	const month = _date.getMonth();
	const year = _date.getFullYear();
	return `${day} ${months[month]}, ${year}`;
};

const getTime = date => {
	const _date = new Date(date);
	const hours = _date.getHours();
	const minutes = _date.getMinutes();
	const seconds = _date.getSeconds();

	const hour = hours > 12 ? hours - 12 : hours;
	return `${hour}:${minutes > 9 ? minutes : '0' + minutes}:${seconds > 9 ? seconds : '0' + seconds} ${hours > 12 ? 'AM' : 'PM'}`;
};

export const AppPortal = ({ token, groupId, appId }) => {
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

	const APIURL = `${location.port === '3000' ? 'http://localhost:5005' : location.origin}/ui`;
	const request = _axios.create({
		baseURL: APIURL,
		headers: {
			Authorization: `Bearer ${token}`
		}
	});

	request.interceptors.response.use(
		response => {
			return response;
		},
		error => {
			if (error.response.status === 401 && error.response.config.url !== '/auth/login') logout();
			return Promise.reject(error);
		}
	);

	const showNotification = ({ message }) => {
		if (!message) return;

		const notificationElement = document.querySelector('#app-notification');
		document.querySelector('#app-notification').style.bottom = '50px';
		notificationElement.innerHTML = message;

		setTimeout(() => {
			document.querySelector('#app-notification').style.bottom = '-100px';
		}, 3000);
	};

	const setEventsDisplayed = events => {
		const dateCreateds = events.map(event => getDate(event.created_at));
		const uniqueDateCreateds = dateCreateds.filter((c, index) => {
			return dateCreateds.indexOf(c) === index;
		});
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
		const uniqueDateCreateds = dateCreateds.filter((c, index) => {
			return dateCreateds.indexOf(c) === index;
		});
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
					url: `/events?sort=AESC&page=${page || 1}&perPage=20&startDate=${startDate}&endDate=${endDate}&appId=${appId}&groupID=${groupId}`,
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
					url: `/eventdeliveries?sort=AESC&page=${page || 1}&perPage=20&startDate=${startDate}&endDate=${endDate}&appId=${appId}&groupID=${groupId}&eventId=${eventId || eventDeliveryEventId || ''}${
						eventDeliveryStatusFilterString || ''
					}`,
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
					url: `/eventdeliveries?eventId=${eventId}&groupID=${groupId}`,
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
					url: `/apps/${appId}/?groupID=${groupId}`
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
					url: `/eventdeliveries/${eventId}/deliveryattempts?groupID=${groupId}`
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
					url: `/eventdeliveries/${eventId}/resend?groupID=${groupId}`
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
		getAppDetails();
		getEvents({ page: 1 });
		getEventDeliveries({ page: 1 });
	}, [getEvents, getAppDetails, getEventDeliveries]);

	return (
		<div className={styles['app-page']}>
			<div className={styles['app-page--head']}>
				<h3>Endpoint</h3>
			</div>

			<div className={styles['app-page--details']}>
				<div className={`${styles['card']} ${styles['app-page--endpoints']}`}>
					<table>
						<thead>
							<tr className={styles['table--head']}>
								<th scope="col">Endpoint URL</th>
								<th scope="col">Created At</th>
								<th scope="col">Updated At</th>
								<th scope="col">Endpoint Events</th>
								<th scope="col">Status</th>
							</tr>
						</thead>
						<tbody>
							{appDetails?.endpoints.map((endpoint, index) => (
								<tr className={styles['has-border']} key={index}>
									<td className={`${styles['has-long-text']} ${styles['longer']}`}>
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
												<div className={styles.tag} key={idx}>
													{event}
												</div>
											))}
										</div>
									</td>
									<td>
										<div>
											<div className={`${styles.tag} ${styles['tag-- ']} ${endpoint.status === 'active' ? styles['tag--Success'] : styles['tag--Retry']}`}>{endpoint.status}</div>
										</div>
									</td>
								</tr>
							))}
						</tbody>
					</table>
				</div>
			</div>

			<section className={`${styles['card']} ${styles['dashboard--logs']}`}>
				<div className={styles['dashboard--logs--tabs']}>
					<div className={`${styles['dashboard--logs--tabs--head']} ${styles.tabs}`}>
						<div className={styles.tabs}>
							{tabs.map((tab, index) => (
								<button onClick={() => toggleActiveTab(tab)} key={index} className={`${styles.clear} ${styles.tab} ${activeTab === tab ? styles.active : ''}`}>
									{tab}
								</button>
							))}
						</div>

						{activeTab === 'events' && <div className="filter"></div>}
					</div>

					<div className={styles['table']}>
						<div className={displayedEvents.length > 0 && activeTab && activeTab === 'events' ? '' : styles.hidden}>
							<div className={styles['filter']}>
								<button className={`${styles['filter--button']} ${eventDateFilterActive ? styles.active : ''}`} onClick={() => toggleShowEventFilterCalendar(!showEventFilterCalendar)}>
									<img src={CalendarIcon} alt="calender icon" />
									<div>Date</div>
									<img src={AngleArrowDownIcon} alt="arrow down icon" />
								</button>
								{showEventFilterCalendar && (
									<div className={styles['date-filter--container']}>
										<DateRange onChange={item => setEventFilterDates([item.selection])} editableDateInputs={true} moveRangeOnFirstSelection={false} ranges={eventFilterDates} />
										<div className={styles['button-container']}>
											<button
												className={styles['primary']}
												onClick={() => {
													getEvents({ dates: eventFilterDates });
													toggleEventDateFilterActive(true);
												}}>
												Apply
											</button>
											<button
												className={`${styles['primary']} ${styles['outline']}`}
												onClick={() => {
													getEvents({ page: 1 });
													toggleEventDateFilterActive(false);
												}}>
												Clear
											</button>
										</div>
									</div>
								)}

								<button
									className={`${styles['filter--button']} ${styles['primary']} ${styles['events-filter-clear-btn']} ${!eventDateFilterActive ? styles.disabled : ''}`}
									onClick={() => clearEventsFilters()}>
									Clear Filter
								</button>
							</div>

							<hr />

							<div className={`${styles['table--container']} ${styles['smaller-table']}`}>
								<table id="events-table">
									<thead>
										<tr className={styles['table--head']}>
											<th scope="col">Event Type</th>
											<th scope="col">App Name</th>
											<th scope="col">Created At</th>
											<th scope="col"></th>
										</tr>
									</thead>
									<tbody>
										{displayedEvents.map((eventGroup, index) => (
											<React.Fragment key={'eventGroup' + index}>
												<tr className={styles['table--date-row']}>
													<td>
														<div>{eventGroup.date}</div>
													</td>
													<td>
														<div></div>
													</td>
													<td>
														<div></div>
													</td>
													<td>
														<div></div>
													</td>
													<td>
														<div></div>
													</td>
													<td>
														<div></div>
													</td>
												</tr>
												{eventGroup.events.map((event, index) => (
													<tr
														key={'events' + index}
														onClick={() => {
															eventDeliveriesForSidebar({ eventId: event.uid });
															setEventDetailsItem(event);
															Prism.highlightAll();
															console.log(Prism);
														}}
														className={event.uid === eventDetailsItem?.uid ? styles['active'] : ''}
														id={'event' + index}>
														<td>
															<div>
																<div className={styles.tag}>{event.event_type}</div>
															</div>
														</td>

														<td className={styles['has-long-text']}>
															<div>{event.app_metadata.title}</div>
														</td>
														<td>
															<div>{getTime(event.created_at)}</div>
														</td>
														<td>
															<div>
																<button
																	className={`${styles['primary']} ${styles['clear']} ${styles['has-icon']} ${styles['icon-right']}`}
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
													</tr>
												))}
											</React.Fragment>
										))}
									</tbody>
								</table>

								{events.pagination.totalPage > 1 && (
									<div className={`${styles['table--load-more']} ${styles['button-container']} ${styles['margin-top']} ${styles.center}`}>
										<button
											className={`${styles['primary']} ${styles['clear']} ${styles['has-icon']} ${styles['icon-left']} ${
												events.pagination.page === events.pagination.totalPage ? styles.disable_action : ''
											}`}
											disabled={events.pagination.page === events.pagination.totalPage}
											onClick={() => getEvents({ page: events.pagination.page + 1, eventsData: events, dates: eventDateFilterActive ? eventFilterDates : null })}>
											<img src={ArrowDownIcon} alt="arrow down icon" />
											Load more
										</button>
									</div>
								)}
							</div>
						</div>

						<div className={displayedEventDeliveries.length > 0 && activeTab && activeTab === 'event deliveries' ? '' : styles.hidden}>
							<div className={styles['filter']}>
								<button className={`${styles['filter--button']} ${eventDelDateFilterActive ? styles.active : ''}}`} onClick={() => toggleEventDelFilterCalendar(!showEventDelFilterCalendar)}>
									<img src={CalendarIcon} alt="calender icon" />
									<div>Date</div>
									<img src={AngleArrowDownIcon} alt="arrow down icon" />
								</button>
								{showEventDelFilterCalendar && (
									<div className={styles['date-filter--container']}>
										<DateRange onChange={item => setEventDelFilterDates([item.selection])} editableDateInputs={true} moveRangeOnFirstSelection={false} ranges={eventDelFilterDates} />
										<div className={styles['button-container']}>
											<button
												className={styles['primary']}
												onClick={() => {
													getEventDeliveries({ dates: eventFilterDates });
													toggleEventDelDateFilterActive(true);
												}}>
												Apply
											</button>
											<button
												className={`${styles['primary']} ${styles['outline']}`}
												onClick={() => {
													clearEventDeliveriesDateFilter();
													toggleEventDelDateFilterActive(false);
												}}>
												Clear
											</button>
										</div>
									</div>
								)}

								<div className={styles['dropdown']}>
									<button
										className={`${styles['filter--button']} ${styles['dropdown--button']} ${eventDelStatusFilterActive ? styles.active : ''}`}
										onClick={() => toggleShowEventDeliveriesStatusDropdown(!showEventDeliveriesStatusDropdown)}>
										<img src={StatusFilterIcon} alt="status filter icon" />
										<span>Status</span>
										<img src={AngleArrowDownIcon} alt="arrow down icon" />
									</button>
									<div className={`${styles['dropdown--list']} + ${showEventDeliveriesStatusDropdown ? styles.show : ''}`}>
										{eventDeliveryStatuses.map((status, index) => (
											<div className={styles['dropdown--list--item']} key={'status' + index}>
												<input type="checkbox" name={status} value={status} id={status} onChange={e => updateEventDevliveryStatusFilter({ status, input: e })} />
												<label htmlFor={status}>{status}</label>
											</div>
										))}
										<button className={styles['primary']} onClick={() => getEventDeliveries({ page: 1, eventDelFilterStatusList: eventDelFilterStatus })}>
											Apply
										</button>
									</div>
								</div>

								{eventDeliveryEventId && (
									<div className={`${styles['filter--button']} ${styles['event-button']} ${styles.active}`}>
										Event Filtered
										<button
											className={`${styles.primary} ${styles.clear}  ${styles['has-icon']}`}
											onClick={() => {
												setEventDeliveryEventId('');
												getEventDeliveries({ page: 1 });
											}}>
											<img src={CloseIcon} alt="close icon" />
										</button>
									</div>
								)}

								<button
									className={`${styles['filter--button']} ${styles['primary']} ${styles['events-filter-clear-btn']} ${
										!eventDelDateFilterActive && eventDeliveryEventId === '' && !eventDelStatusFilterActive ? styles.disabled : ''
									}`}
									onClick={() => clearEventDeliveriesFilters()}>
									Clear Filter
								</button>
							</div>

							<hr />

							<div className={`${styles['table--container']}`}>
								<table id="events-deliveries-table">
									<thead>
										<tr className={styles['table--head']}>
											<th scope="col" className={styles['checkbox']}>
												<div className={styles['checkbox']}>
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
												<tr className={styles['table--date-row']}>
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
														className={event.uid === eventDelDetailsItem?.uid ? styles['active'] : ''}
														id={'eventDel' + index}
														key={'eventDels' + index}
														onClick={() => {
															Prism.highlightAll();
															setEventDelDetailsItem(event);
															getDelieveryAttempts(event.uid);
														}}>
														<td>
															<div className={`${styles['checkbox']} ${styles['has-retry']}`}>
																{event.metadata?.num_trials > event.metadata?.retry_limit && <img src={RetryIcon} alt="retry icon" title="manual retried" />}
																<input type="checkbox" id="event" />
																<div className={`${styles.tag} ${styles['tag--' + event.status]}`}>{event.status}</div>
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
																	className={`${styles['primary']} ${styles['has-icon']} ${styles['icon-left']}`}
																	disabled={event.status === 'Success' || event.status === 'Scheduled'}
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
									<div className={`${styles['table--load-more']} ${styles['button-container']} ${styles['margin-top']} ${styles.center}`}>
										<button
											className={`${styles['primary']} ${styles['clear']} ${styles['has-icon']} ${styles['icon-left']} ${
												eventDeliveries.pagination.page === eventDeliveries.pagination.totalPage ? styles.disable_action : ''
											}`}
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
							<div className={styles['empty-state']}>
								<img src={EmptyStateImage} alt="empty state" />
								<p>No {activeTab} to show</p>
							</div>
						)}

						{activeTab === 'event deliveries' && displayedEventDeliveries.length === 0 && (
							<div className={styles['empty-state']}>
								<img src={EmptyStateImage} alt="empty state" />
								<p>No {activeTab} to show</p>
							</div>
						)}
					</div>
				</div>

				<div className={styles['dashboard--logs--details']}>
					<div className={eventDetailsItem && activeTab === 'events' ? '' : styles.hidden}>
						<h3>Details</h3>

						<div className={styles['dashboard--logs--details--req-res']}>
							<div className={`${styles['dashboard--logs--details--tabs-data']} ${styles.show}`}>
								<h3>Event</h3>
								<pre className={`${styles['line-numbers']} ${styles['lang-javascript']}`}>
									<code className={styles['lang-javascript']}>{eventDetailsItem?.data ? JSON.stringify(eventDetailsItem.data, null, 4).replaceAll(/"([^"]+)":/g, '$1:') : ''}</code>
								</pre>
							</div>
						</div>

						<h4>Deliveries Overview</h4>
						<ul className={`${styles['dashboard--logs--details--endpoints']} ${styles.inline}`}>
							{eventDeliveriesSidebar.length > 0 &&
								eventDeliveriesSidebar.map((event, index) => (
									<li key={'endpoint' + index}>
										<div className={`${styles.tag} ${styles['tag--' + event.status]}`}>{event.status}</div>
										<div className={styles.url} title={event.endpoint.target_url}>
											{event.endpoint.target_url}
										</div>
									</li>
								))}
						</ul>
					</div>

					<div className={eventDelDetailsItem && activeTab === 'event deliveries' ? '' : styles.hidden}>
						<h3>Details</h3>
						<ul className={styles['dashboard--logs--details--meta']}>
							<li className={styles['list-item-inline']}>
								<div className={styles['list-item-inline--label']}>IP Address</div>
								<div className={`${styles['list-item-inline--item']} ${styles.color}`}>{eventDeliveryAtempt?.ip_address || '-'}</div>
							</li>
							<li className={styles['list-item-inline']}>
								<div className={styles['list-item-inline--label']}>HTTP Status</div>
								<div className={styles['list-item-inline--item']}>{eventDeliveryAtempt?.http_status || '-'}</div>
							</li>
							<li className={styles['list-item-inline']}>
								<div className={styles['list-item-inline--label']}>API Version</div>
								<div className={styles['list-item-inline--item']}>{eventDeliveryAtempt?.api_version || '-'}</div>
							</li>
							<li className={styles['list-item-inline']}>
								<div className={styles['list-item-inline--label']}>Endpoint</div>
								<div className={styles['list-item-inline--item']}>{eventDelDetailsItem?.endpoint?.target_url}</div>
							</li>
							<li className={styles['list-item-inline']}>
								<div className={styles['list-item-inline--label']}>Next Retry</div>
								<div className={styles['list-item-inline--item']}>{getTime(eventDelDetailsItem?.metadata.next_send_time)}</div>
							</li>
							<li className={styles['list-item-inline']}>
								<div className={styles['list-item-inline--label']}>App Name</div>
								<div className={styles['list-item-inline--item']}>{eventDelDetailsItem?.app_metadata.title}</div>
							</li>
							<li className={styles['list-item-inline']}>
								<div className={styles['list-item-inline--label']}>Delivery Time</div>
								<div className={styles['list-item-inline--item']}>{getTime(eventDelDetailsItem?.updated_at)}</div>
							</li>
						</ul>

						<ul className={styles.tabs}>
							{eventDetailsTabs.map(tab => (
								<li className={`${styles.tab} ${eventDetailsActiveTab === tab.id ? styles.active : ''}`} key={tab.id}>
									<button className={`${styles.primary} ${styles.outline}`} onClick={() => setEventDetailsActiveTab(tab.id)}>
										{tab.label}
									</button>
								</li>
							))}
						</ul>

						<div className={styles['dashboard--logs--details--req-res']}>
							<div className={`${styles['dashboard--logs--details--tabs-data']} ${eventDetailsActiveTab === 'data' ? styles.show : ''}`}>
								<pre className={`${styles['line-numbers']} ${styles['lang-javascript']}`}>
									<code className={styles['lang-javascript']}>
										{eventDelDetailsItem?.metadata.data ? JSON.stringify(eventDelDetailsItem.metadata.data, null, 4).replaceAll(/"([^"]+)":/g, '$1:') : ''}
									</code>
								</pre>
							</div>

							<div className={`${styles['dashboard--logs--details--tabs-data']} ${eventDetailsActiveTab === 'response' ? styles.show : ''}`}>
								<h3>Header</h3>
								<pre className={`${styles['line-numbers']} ${styles['lang-javascript']}`}>
									<code className={styles['lang-javascript']}>
										{eventDeliveryAtempt?.response_http_header
											? JSON.stringify(eventDeliveryAtempt.response_http_header, null, 4).replaceAll(/"([^"]+)":/g, '$1:')
											: 'No response header was sent'}
									</code>
								</pre>

								<h3>Body</h3>
								<pre className={`${styles['line-numbers']} ${styles['lang-javascript']}`}>
									<code className={styles['lang-html']}>{eventDeliveryAtempt?.response_data ? eventDeliveryAtempt.response_data : 'No response body was sent'}</code>
								</pre>
							</div>

							<div className={`${styles['dashboard--logs--details--tabs-data']} ${eventDetailsActiveTab === 'request' ? styles.show : ''}`}>
								<h3>Header</h3>
								<pre className={`${styles['line-numbers']} ${styles['lang-javascript']}`}>
									<code className={styles['lang-javascript']}>
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
			<div className={styles['app-notification']} id="app-notification"></div>
		</div>
	);
};
