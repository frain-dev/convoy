import React, { useEffect, useState, useCallback } from 'react';
import ArrowDownIcon from 'assets/img/arrow-down-icon.svg';
import CloseIcon from 'assets/img/close-icon.svg';
import RefreshIcon from 'assets/img/refresh-icon-primary.svg';
import CalendarIcon from 'assets/img/calendar-icon.svg';
import BatchRetryGif from 'assets/img/filter.gif';
import AngleArrowDownIcon from 'assets/img/angle-arrow-down.svg';
import RotateIcon from 'assets/img/rotate-icon.svg';
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

export const ConvoyApp = ({ token, apiURL }) => {
	const [eventDeliveryEventId, setEventDeliveryEventId] = useState('');
	const [eventsToRetry, setEventsToRetry] = useState('');
	const [isRetryingBatchEvents, toggleBatchRetryLoadStatus] = useState('');
	const [showBatchRetryModal, toggleBatchRetryModal] = useState(false);
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
	const [isloadingMoreEvents, toggleIsloadingMoreEvents] = useState(false);
	const [isloadingMoreEventDels, toggleIsloadingMoreEventDels] = useState(false);
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

	apiURL = apiURL + '/portal';
	const request = _axios.create({
		baseURL: apiURL,
		headers: {
			Authorization: `Bearer ${token}`
		}
	});

	request.interceptors.response.use(
		response => {
			return response;
		},
		error => {
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

	const clearEventDeliveriesDateFilter = ({ appUid }) => {
		setEventDelFilterDates([
			{
				startDate: new Date(),
				endDate: new Date(),
				key: 'selection'
			}
		]);
		toggleEventDelDateFilterActive(false);
		getEventDeliveries({ page: 1, appUid: appUid });
	};

	const clearEventDeliveriesFilters = ({ appUid }) => {
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
		getEventDeliveries({ page: 1, eventDelFilterStatusList: [], appUid: appUid });
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

	const fetchBatchRetryEventNo = async ({ dates, eventDelFilterStatusList = [] }) => {
		let eventDeliveryStatusFilterString = '';
		eventDelFilterStatusList.forEach(status => (eventDeliveryStatusFilterString += `&status=${status}`));

		if (!dates) dates = [{ startDate: null, endDate: null }];

		const dateFromPicker = dates[0];
		const { startDate, endDate } = setDateForFilter(dateFromPicker);

		try {
			const batchRetryResponse = await (
				await request({
					url: `/eventdeliveries/countbatchretryevents?startDate=${startDate}&endDate=${endDate}&eventId=${eventDeliveryEventId || ''}${eventDeliveryStatusFilterString || ''}`,
					method: 'GET',
					data: null
				})
			).data;

			setEventsToRetry(batchRetryResponse.data.num);
			toggleBatchRetryModal(true);
		} catch (error) {
			showNotification({ message: error.response.data.message });
			return error;
		}
	};

	const batchRetryEvents = async ({ dates, eventDelFilterStatusList = [], appUid }) => {
		let eventDeliveryStatusFilterString = '';
		eventDelFilterStatusList.forEach(status => (eventDeliveryStatusFilterString += `&status=${status}`));
		toggleBatchRetryLoadStatus(true);
		if (!dates) dates = [{ startDate: null, endDate: null }];

		const dateFromPicker = dates[0];
		const { startDate, endDate } = setDateForFilter(dateFromPicker);

		try {
			await (
				await request({
					url: `/eventdeliveries/batchretry?startDate=${startDate}&endDate=${endDate}&eventId=${eventDeliveryEventId || ''}${eventDeliveryStatusFilterString || ''}`,
					method: 'POST',
					data: null
				})
			).data;

			getEventDeliveries({ page: 1, appUid: appUid });
			toggleBatchRetryLoadStatus(false);
			toggleBatchRetryModal(false);
			showNotification({ message: 'Batch retry successful' });
		} catch (error) {
			toggleBatchRetryLoadStatus(false);
			toggleBatchRetryModal(false);
			showNotification({ message: error.response.data.message });
			return error;
		}
	};

	const getEvents = useCallback(async ({ page, eventsData, dates, appUid }) => {
		toggleShowEventFilterCalendar(false);
		if (eventsData && eventsData?.pagination?.next === page) toggleIsloadingMoreEvents(true);

		if (!dates) dates = [{ startDate: null, endDate: null }];

		const dateFromPicker = dates[0];
		const { startDate, endDate } = setDateForFilter(dateFromPicker);

		try {
			const eventsResponse = await (
				await request({
					url: `/events?sort=AESC&page=${page || 1}&perPage=20&startDate=${startDate}&endDate=${endDate}&appId=${appUid}`,
					method: 'GET'
				})
			).data;

			if (eventsData && eventsData?.pagination?.next === page) {
				const content = [...eventsData.content, ...eventsResponse.data.content];
				const pagination = eventsResponse.data.pagination;
				setEventsData({ content, pagination });
				setEventsDisplayed(content);
				toggleIsloadingMoreEvents(false);
				return;
			}

			setEventsData(eventsResponse.data);
			setEventsDisplayed(eventsResponse.data.content);
			setEventDetailsItem(eventsResponse.data.content[0]);
			eventDeliveriesForSidebar({ eventId: eventsResponse.data.content[0].uid, appUid: appUid });
			Prism.highlightAll();
		} catch (error) {
			return error;
		}
	}, []);

	const getEventDeliveries = useCallback(async ({ page, eventsData, dates, eventId, eventDelFilterStatusList = [], appUid }) => {
		if (eventsData && eventsData?.pagination?.next === page) toggleIsloadingMoreEventDels(true);
		let eventDeliveryStatusFilterString = '';
		toggleEventDelFilterCalendar(false);
		toggleShowEventDeliveriesStatusDropdown(false);
		toggleEventDelStatusFilterActive(eventDelFilterStatusList.length > 0);
		eventDelFilterStatusList.forEach(status => (eventDeliveryStatusFilterString += `&status=${status}`));

		if (!dates) dates = [{ startDate: null, endDate: null }];

		const dateFromPicker = dates[0];
		const { startDate, endDate } = setDateForFilter(dateFromPicker);

		try {
			const eventsResponse = await (
				await request({
					url: `/eventdeliveries?sort=AESC&page=${page || 1}&perPage=20&startDate=${startDate}&endDate=${endDate}&appId=${appUid}&eventId=${eventId || eventDeliveryEventId || ''}${
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
				toggleIsloadingMoreEventDels(false);
				return;
			}

			setEventDeliveriesData(eventsResponse.data);
			setEventDeliveriesDisplayed(eventsResponse.data.content);
			setEventDelDetailsItem(eventsResponse.data.content[0]);
			getDelieveryAttempts({ eventId: eventsResponse.data.content[0].uid, appUid: appUid });
			Prism.highlightAll();
		} catch (error) {
			return error;
		}
	}, []);

	const eventDeliveriesForSidebar = useCallback(async ({ eventId, appUid }) => {
		try {
			const eventsResponse = await (
				await request({
					url: `/eventdeliveries?eventId=${eventId}&appId=${appUid}`,
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
					url: `/apps`
				})
			).data;
			setAppDetails(appDetailsResponse.data);
			const appUid = appDetailsResponse.data.uid;
			getEvents({ page: 1, appUid: appUid });
			getEventDeliveries({ page: 1, appUid: appUid });
		} catch (error) {
			return error;
		}
	}, []);

	const getDelieveryAttempts = async ({ eventId, appUid }) => {
		try {
			const deliveryAttemptsResponse = await (
				await request({
					url: `/eventdeliveries/${eventId}/deliveryattempts?appId=${appUid}`
				})
			).data;
			setEventDeliveryAtempt(deliveryAttemptsResponse.data[deliveryAttemptsResponse.data.length - 1]);
			Prism.highlightAll();
		} catch (error) {
			return error;
		}
	};

	const retryEvent = async ({ eventId, e, index, appUid }) => {
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
			getEvents({ page: events.pagination.page, appUid: appUid });
		} catch (error) {
			showNotification({ message: error.response.data.message });
			retryButton.classList.remove(['spin', 'disable_action']);
			retryButton.disabled = false;
			return error;
		}
	};

	const forceRetryEvent = async ({ eventId, e, index, appUid }) => {
		e.stopPropagation();
		const retryButton = document.querySelector(`#eventDel${index} button`);
		retryButton.classList.add(['spin', 'disable_action']);
		retryButton.disabled = true;
		const payload = {
			ids: [eventId]
		};
		try {
			await (
				await request({
					method: 'POST',
					url: `/eventdeliveries/forceresend`,
					body: payload
				})
			).data;
			showNotification({ message: 'Force Retry Request Sent' });
			retryButton.classList.remove(['spin', 'disable_action']);
			retryButton.disabled = false;
			getEvents({ page: events.pagination.page, appUid: appUid });
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
	}, [getAppDetails]);

	return (
		<div className={styles['dashboard--page']}>
			<div className={styles['dashboard--page--head']}>
				<h3 className={styles['margin-bottom__10px']}>Endpoints</h3>
			</div>

			<div className={styles['app-page--details']}>
				<div className={`${styles['card']} ${styles['has-title']} ${styles['dashboard-page--endpoints']}`}>
					{appDetails?.endpoints?.length !== 0 && (
						<table className={`${styles.table} ${styles['table__no-style']}`}>
							<thead>
								<tr className={styles['table--head']}>
									<th className={styles['has-long-text']} scope="col">
										Endpoint URL
									</th>
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
											<div className={`${styles.flex} ${styles['flex__wrap']}`}>
												{endpoint.events.map((event, idx) => (
													<div className={styles.tag} key={idx}>
														{event == '*' ? 'all events' : event}
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
					)}

					{appDetails?.endpoints?.length === 0 && (
						<div className={`${styles['empty-state']} ${styles['table--container']} ${styles['smaller-table']}`}>
							<img src={EmptyStateImage} alt="empty state" />
							<p>No endpoints to show</p>
						</div>
					)}
				</div>
			</div>

			<section className={`${styles['card']} ${styles['dashboard--logs']} ${styles['has-title']}`}>
				<div className={styles['dashboard--logs--tabs']}>
					<div className={styles.tabs}>
						{tabs.map((tab, index) => (
							<li key={index}>
								<button onClick={() => toggleActiveTab(tab)} className={`${activeTab === tab ? styles.active : ''}`}>
									<span>{tab}</span>
								</button>
							</li>
						))}
					</div>
				</div>

				<div className={styles['dashboard--logs--filter']}>
					<div className={activeTab === 'events' ? '' : styles.hidden}>
						<div className={`${styles.flex} ${styles['flex__align-items-center']} ${styles['flex__justify-between']}`}>
							<div className={styles['dropdown']}>
								<button
									className={`${styles['button']} ${styles['button__filter']} ${styles['without-margin']} ${styles['button--has-icon']} ${styles['icon-left']} ${styles['icon-right']} ${
										eventDateFilterActive ? styles.active : ''
									}`}
									onClick={() => toggleShowEventFilterCalendar(!showEventFilterCalendar)}>
									<img src={CalendarIcon} alt="calender icon" />
									<div>Date</div>
									<img src={AngleArrowDownIcon} alt="arrow down icon" />
								</button>

								{showEventFilterCalendar && (
									<div className={`${styles['dropdown__menu']} ${styles['with-padding']} ${showEventFilterCalendar ? styles.show : ''}`}>
										<DateRange onChange={item => setEventFilterDates([item.selection])} editableDateInputs={true} moveRangeOnFirstSelection={false} ranges={eventFilterDates} />
										<div className={`${styles['flex']} ${styles['flex__align-items-center']} ${styles['margin-top__10px']}`}>
											<button
												className={`${styles['button']} ${styles['button__small']} ${styles['button__primary']} ${styles['font__12px']} ${styles['margin-right__10px']}`}
												onClick={() => {
													getEvents({ dates: eventFilterDates, appUid: appDetails?.uid });
													toggleEventDateFilterActive(true);
												}}>
												Apply
											</button>
											<button
												className={`${styles['button__clear']}`}
												onClick={() => {
													getEvents({ page: 1, appUid: appDetails?.uid });
													toggleEventDateFilterActive(false);
												}}>
												Clear
											</button>
										</div>
									</div>
								)}
							</div>
						</div>
					</div>

					<div className={activeTab === 'event deliveries' ? '' : styles.hidden}>
						<div className={`${styles.flex} ${styles['flex__align-items-center']} ${styles['flex__justify-between']}`}>
							<div className={styles['flex']}>
								<div className={styles['dropdown']}>
									<button
										className={`${styles['button']} ${styles['button__filter']} ${styles['without-margin']} ${styles['button--has-icon']} ${styles['icon-left']} ${styles['icon-right']} ${
											eventDelDateFilterActive ? styles.active : ''
										}`}
										onClick={() => toggleEventDelFilterCalendar(!showEventDelFilterCalendar)}>
										<img src={CalendarIcon} alt="calender icon" />
										<div>Date</div>
										<img src={AngleArrowDownIcon} alt="arrow down icon" />
									</button>
									{showEventDelFilterCalendar && (
										<div className={`${styles['dropdown__menu']} ${styles['with-padding']} ${showEventDelFilterCalendar ? styles.show : ''} `}>
											<DateRange onChange={item => setEventDelFilterDates([item.selection])} editableDateInputs={true} moveRangeOnFirstSelection={false} ranges={eventDelFilterDates} />
											<div className={styles['button-container']}>
												<button
													className={`${styles['button']} ${styles['button__small']} ${styles['button__primary']} ${styles['font__12px']} ${styles['margin-right__10px']}`}
													onClick={() => {
														getEventDeliveries({ dates: eventFilterDates, appUid: appDetails?.uid });
														toggleEventDelDateFilterActive(true);
													}}>
													Apply
												</button>
												<button
													className={`${styles['button__clear']}`}
													onClick={() => {
														clearEventDeliveriesDateFilter({ appUid: appDetails?.uid });
														toggleEventDelDateFilterActive(false);
													}}>
													Clear
												</button>
											</div>
										</div>
									)}
								</div>

								<div className={styles['dropdown']}>
									<button
										className={`${styles['button']} ${styles['button__filter']} ${styles['button--has-icon']} ${styles['icon-right']} ${styles['icon-left']} ${styles['margin-left__24px']} ${
											eventDelStatusFilterActive ? styles.active : ''
										}`}
										onClick={() => toggleShowEventDeliveriesStatusDropdown(!showEventDeliveriesStatusDropdown)}>
										<img src={StatusFilterIcon} alt="status filter icon" />
										<span>Status</span>
										<img src={AngleArrowDownIcon} alt="arrow down icon" />
									</button>
									<div className={`${styles['dropdown__menu']} ${styles['with-padding']} ${styles.small} ${showEventDeliveriesStatusDropdown ? styles.show : ''}`}>
										{eventDeliveryStatuses.map((status, index) => (
											<div className={`${styles['dropdown__menu__item']} ${styles['with-border']}`} key={'status' + index}>
												<label htmlFor={status}>{status}</label>
												<input type="checkbox" name={status} value={status} id={status} onChange={e => updateEventDevliveryStatusFilter({ status, input: e })} />
											</div>
										))}
										<div className={`${styles['flex']} ${styles['flex__align-items-center']} ${styles['margin-top__12px']}`}>
											<button
												className={`${styles['button']} ${styles['button__primary']} ${styles['button__small']}`}
												onClick={() => getEventDeliveries({ page: 1, eventDelFilterStatusList: eventDelFilterStatus, appUid: appDetails?.uid })}>
												Apply
											</button>
											<button className={`${styles['button__clear']} ${styles['margin-left__14px']}`} onClick={() => getEventDeliveries({ page: 1, appUid: appDetails?.uid })}>
												Clear
											</button>
										</div>
									</div>
								</div>

								{eventDeliveryEventId && (
									<div className={`${styles['button__filter']} ${styles['margin-left__24px']} ${styles.active}`}>
										Event Filtered
										<button
											className={`${styles['button__clear']} ${styles['button--has-icon']}  ${styles['margin-left__8px']}`}
											onClick={() => {
												setEventDeliveryEventId('');
												getEventDeliveries({ page: 1, appUid: appDetails?.uid });
											}}>
											<img src={CloseIcon} alt="close icon" />
										</button>
									</div>
								)}

								<button
									className={`${styles['button']} ${styles['button__filter']} ${styles['margin-left__24px']}`}
									disabled={!eventDelDateFilterActive && !eventDelStatusFilterActive}
									onClick={() => fetchBatchRetryEventNo({ dates: eventDelDateFilterActive ? eventFilterDates : null, eventDelFilterStatusList: eventDelFilterStatus })}>
									<span>Batch Retry</span>
								</button>
							</div>

							<button
								className={`${styles['button']} ${styles['button__white']} ${styles['button__small']} ${styles['font__12px']} ${styles['margin-right__20px']} ${
									!eventDelDateFilterActive && eventDeliveryEventId === '' && !eventDelStatusFilterActive ? styles.disabled : ''
								}`}
								onClick={() => clearEventDeliveriesFilters({ appUid: appDetails?.uid })}>
								Clear Filter
							</button>
						</div>
					</div>
				</div>

				<div className={styles['flex']}>
					<div className={styles['dashboard--logs--table']}>
						<div className={`${styles['table']} ${styles['table--container']} ${styles['has-loader']} ${displayedEvents.length > 0 && activeTab && activeTab === 'events' ? '' : styles.hidden}`}>
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
														eventDeliveriesForSidebar({ eventId: event.uid, appUid: appDetails?.uid });
														setEventDetailsItem(event);
														Prism.highlightAll();
														console.log(Prism);
													}}
													className={`${event.uid === eventDetailsItem?.uid ? styles.active : ''} ${index === eventGroup.events.length - 1 ? styles['last-item'] : ''}`}
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
																className={`${styles['button']} ${styles['button__clear']} ${styles['button--has-icon']} ${styles['icon-right']}`}
																onClick={e => {
																	e.stopPropagation();
																	setEventDeliveryEventId(event.uid);
																	getEventDeliveries({ page: 1, eventId: event.uid, appUid: appDetails?.uid });
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
								<div className={`${styles['table--load-more']} ${styles['button--container']} ${styles.center}`}>
									<button
										className={`${styles['button']} ${styles['button__clear']} ${styles['button--has-icon']} ${styles['icon-left']} ${styles['margin-top__20px']} ${
											styles['margin-bottom__24px']
										} ${styles['flex__justify-center']} ${events.pagination.page === events.pagination.totalPage ? styles.disable_action : ''}`}
										disabled={events.pagination.page === events.pagination.totalPage}
										onClick={() => getEvents({ page: events.pagination.page + 1, eventsData: events, dates: eventDateFilterActive ? eventFilterDates : null, appUid: appDetails?.uid })}>
										{!isloadingMoreEvents && <img src={ArrowDownIcon} className={`${styles['width-unset']} ${styles['height-unset']}`} alt="arrow down icon" />}
										{isloadingMoreEvents && <img src={RotateIcon} className={styles['loading-icon']} alt="loading icon" />}
										Load more
									</button>
								</div>
							)}
						</div>

						<div
							className={`${styles['table']} ${styles['table--container']} ${styles['has-loader']} ${
								displayedEventDeliveries.length > 0 && activeTab && activeTab === 'event deliveries' ? '' : styles.hidden
							}`}>
							<table id="events-deliveries-table">
								<thead>
									<tr className={styles['table--head']}>
										<th scope="col">Status</th>
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
													className={`${event.uid === eventDelDetailsItem?.uid ? styles['active'] : ''} ${index === eventDeliveriesGroup.events.length - 1 ? styles['last-item'] : ''}`}
													id={'eventDel' + index}
													key={'eventDels' + index}
													onClick={() => {
														Prism.highlightAll();
														setEventDelDetailsItem(event);
														getDelieveryAttempts({ eventId: event.uid, appUid: appDetails?.uid });
													}}>
													<td>
														<div className={`${styles['has-retry']}`}>
															{event.metadata?.num_trials > event.metadata?.retry_limit && <img src={RetryIcon} alt="retry icon" title="manual retried" />}
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
																className={event.status !== 'Success' ? `${styles['button__retry']} ${styles['button--has-icon']} ${styles['icon-left']}` : styles.hidden}
																disabled={event.status !== 'Failure'}
																onClick={e => retryEvent({ eventId: event.uid, e, index, appUid: appDetails?.uid })}>
																<img src={RefreshIcon} alt="refresh icon" />
																Retry
															</button>
															<button
																className={event.status === 'Success' ? `${styles['button__retry']} ${styles['button--has-icon']} ${styles['icon-left']}` : styles.hidden}
																onClick={e => forceRetryEvent({ eventId: event.uid, e, index, appUid: appDetails?.uid })}>
																<img src={RefreshIcon} alt="refresh icon" />
																Force Retry
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
								<div className={`${styles['table--load-more']} ${styles['button--container']} ${styles.center}`}>
									<button
										className={`${styles['button']} ${styles['button__clear']} ${styles['button--has-icon']} ${styles['icon-left']} ${styles['margin-top__24px']} ${
											styles['margin-bottom__24px']
										} ${styles['flex__justify-center']} ${eventDeliveries.pagination.page === eventDeliveries.pagination.totalPage ? styles.disable_action : ''}`}
										disabled={eventDeliveries.pagination.page === eventDeliveries.pagination.totalPage}
										onClick={() =>
											getEventDeliveries({
												page: eventDeliveries.pagination.page + 1,
												eventsData: eventDeliveries,
												dates: eventDelDateFilterActive ? eventDelFilterDates : null,
												eventDelFilterStatusList: eventDelStatusFilterActive ? eventDelFilterStatus : null,
												appUid: appDetails?.uid
											})
										}>
										{!isloadingMoreEventDels && <img src={ArrowDownIcon} className={`${styles['width-unset']} ${styles['height-unset']}`} alt="arrow down icon" />}
										{isloadingMoreEventDels && <img src={RotateIcon} className={styles['loading-icon']} alt="loading icon" />}
										Load more
									</button>
								</div>
							)}
						</div>

						{activeTab === 'events' && displayedEvents.length === 0 && (
							<div className={`${styles['empty-state']} ${styles['table--container']}`}>
								<img src={EmptyStateImage} alt="empty state" />
								<p>No {activeTab} to show</p>
							</div>
						)}

						{activeTab === 'event deliveries' && displayedEventDeliveries.length === 0 && (
							<div className={`${styles['empty-state']} ${styles['table--container']}`}>
								<img src={EmptyStateImage} alt="empty state" />
								<p>No {activeTab} to show</p>
							</div>
						)}
					</div>

					<div className={`${styles['dashboard--logs--details']} ${styles['has-loader']}`}>
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

							<ul className={`${styles.tabs} ${styles.tabs__logs}`}>
								{eventDetailsTabs.map(tab => (
									<li className={`${eventDetailsActiveTab === tab.id ? styles.active : ''}`} key={tab.id}>
										<button onClick={() => setEventDetailsActiveTab(tab.id)}>{tab.label}</button>
									</li>
								))}
							</ul>

							<div className={`${styles['dashboard--logs--details--req-res']} ${styles['has-loader']}`}>
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
				</div>
			</section>
			<div className={styles['app-notification']} id="app-notification"></div>

			{(showEventFilterCalendar || showEventDelFilterCalendar || showEventDeliveriesStatusDropdown) && (
				<div
					className={styles['overlay']}
					onClick={() => {
						toggleShowEventFilterCalendar(false);
						toggleEventDelFilterCalendar(false);
						toggleShowEventDeliveriesStatusDropdown(false);
					}}></div>
			)}

			{showBatchRetryModal && (
				<React.Fragment>
					<div className={styles._overlay} onClick={() => toggleBatchRetryModal(false)}></div>
					<div className={`${styles.modal} ${styles.modal__center}`}>
						<div className={`${styles['modal--body']} ${styles.flex} ${styles.flex__column} ${styles['flex__justify-center']} ${styles['flex__align-items-center']} ${styles['delete']}`}>
							<img width="80" src={BatchRetryGif} alt="batch retry gif" className={styles['filter-img']} />
							<div className={`${styles['text-center']} ${styles['font__20px']} ${styles['font__weight-500']} ${styles['color__grey']} ${styles['margin-bottom__8px']}`}>
								The filters applied will affect
							</div>
							<div className={`${styles['text-center']} ${styles['font__20px']} ${styles['font__weight-600']} ${styles['color__black']} ${styles['margin-bottom__32px']}`}>
								{eventsToRetry || 0} event{eventsToRetry > 1 ? 's' : ''}
							</div>
							<button
								className={`${styles.button} ${styles.button__primary}`}
								disabled={isRetryingBatchEvents || eventsToRetry == 0}
								onClick={() => batchRetryEvents({ dates: eventDelDateFilterActive ? eventFilterDates : null, eventDelFilterStatusList: eventDelFilterStatus, appUid: appDetails?.uid })}>
								{isRetryingBatchEvents ? 'Retrying Events...' : 'Yes, Retry Events'}
							</button>
							<button className={`${styles.button__primary} ${styles.button__clear} ${styles['margin-top__22px']} ${styles['font__weight-600']}`} onClick={() => toggleBatchRetryModal(false)}>
								No, Cancel
							</button>
						</div>
					</div>
				</React.Fragment>
			)}
		</div>
	);
};
