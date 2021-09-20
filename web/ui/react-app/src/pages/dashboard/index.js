import React, { useEffect, useState, useCallback } from 'react';
import * as axios from 'axios';
import ArrowDownIcon from '../../assets/img/arrow-down-icon.svg';
import AppsIcon from '../../assets/img/apps-icon.svg';
import MessageIcon from '../../assets/img/message-icon.svg';
import RefreshIcon from '../../assets/img/refresh-icon.svg';
import CalendarIcon from '../../assets/img/calendar-icon.svg';
import CopyIcon from '../../assets/img/copy-icon.svg';
import LinkIcon from '../../assets/img/link-icon.svg';
import AngleArrowLeftIcon from '../../assets/img/angle-arrow-left.svg';
import AngleArrowRightIcon from '../../assets/img/angle-arrow-right.svg';
import AngleArrowDownIcon from '../../assets/img/angle-arrow-down.svg';
import AngleArrowUpIcon from '../../assets/img/angle-arrow-up.svg';
import ConvoyLogo from '../../assets/img/logo.svg';
import Chart from 'chart.js/auto';
import { DateRange } from 'react-date-range';
import ReactJson from 'react-json-view';
import { AuthDetails, APIURL } from '../../helpers/get-details';
import './style.scss';
import 'react-date-range/dist/styles.css';
import 'react-date-range/dist/theme/default.css';

const _axios = axios.default;
const request = _axios.create({
	baseURL: APIURL,
	headers: {
		Authorization: `Bearer ${AuthDetails().token}`,
	},
});
const months = ['Jan', 'Feb', 'Mar', 'April', 'May', 'June', 'July', 'Aug', 'Sept', 'Oct', 'Nov', 'Dec'];

function DashboardPage() {
	const [dashboardData, setDashboardData] = useState({ apps: 0, messages: 0, messageData: [] });
	const [viewAllEventData, toggleViewAllEventDataState] = useState(false);
	const [viewAllResponseData, toggleViewAllResponseData] = useState(false);
	const [apps, setAppsData] = useState({ content: [], pagination: { page: 1, totalPage: 0 } });
	const [events, setEventsData] = useState({ content: [], pagination: { page: 1, totalPage: 0 } });
	const [tabs] = useState(['events', 'apps']);
	const [activeTab, setActiveTab] = useState('events');
	const [showFilterCalendar, toggleShowFilterCalendar] = useState(false);
	const [organisations, setOrganisations] = useState([]);
	const [activeorganisation, setActiveOrganisation] = useState({
		uid: '',
		name: '',
		created_at: 0,
		updated_at: 0,
		deleted_at: 0,
	});
	const [eventDeliveryAtempt, setEventDeliveryAtempt] = useState({
		ip_address: '',
		http_status: '',
		api_version: '',
		updated_at: 0,
		deleted_at: 0,
		response_data: '',
	});
	const [detailsItem, setDetailsItem] = useState();
	const [filterFrequency, setFilterFrequency] = useState('daily');
	const [filterDates, setFilterDates] = useState([
		{
			startDate: new Date(new Date().setDate(new Date().getDate() - 20)),
			endDate: new Date(),
			key: 'selection',
		},
	]);

	const [jsonStyle] = useState({
		fontSize: '12px',
		lineHeight: '25px',
	});

	const [options] = useState({
		plugins: {
			legend: {
				display: false,
			},
		},
		scales: {
			xAxis: {
				display: true,
				grid: {
					display: false,
				},
			},
		},
	});

	const getDate = (date) => {
		const _date = new Date(date);
		const day = _date.getDate();
		const month = _date.getMonth();
		const year = _date.getFullYear();
		return `${day} ${months[month]}, ${year}`;
	};

	const copyText = (copyText) => {
		const el = document.createElement('textarea');
		el.value = copyText;
		document.body.appendChild(el);
		el.select();
		document.execCommand('copy');
		el.style.display = 'none';
	};

	const getEvents = useCallback(
		async ({ page }) => {
			try {
				const appsResponse = await (
					await request({
						url: `/events?sort=AESC&page=${page || 1}&perPage=10&orgId=${activeorganisation.uid}`,
						method: 'GET',
					})
				).data;
				setEventsData(appsResponse.data);
			} catch (error) {
				return error;
			}
		},
		[activeorganisation],
	);

	const getApps = useCallback(
		async ({ page }) => {
			try {
				const appsResponse = await (
					await request({
						url: `/apps?sort=AESC&page=${page || 1}&perPage=10&orgId=${activeorganisation.uid}`,
					})
				).data;
				setAppsData(appsResponse.data);
			} catch (error) {
				return error;
			}
		},
		[activeorganisation],
	);

	const getDelieveryAttempts = async (eventId) => {
		try {
			const deliveryAttemptsResponse = await (
				await request({
					url: `/events/${eventId}/deliveryattempts`,
				})
			).data;
			setEventDeliveryAtempt(deliveryAttemptsResponse.data[deliveryAttemptsResponse.data.length - 1]);
		} catch (error) {
			return error;
		}
	};

	const retryEvent = async ({ eventId, appId }) => {
		try {
			await (
				await request({
					method: 'PUT',
					url: `/apps/${appId}/events/${eventId}/resend`,
				})
			).data;
		} catch (error) {
			return error;
		}
	};

	const logout = () => {
		localStorage.removeItem('CONVOY_AUTH');
		window.location.replace('/login');
	};

	useEffect(() => {
		const getOrganisations = async () => {
			try {
				const organisationsResponse = await (
					await request({
						url: '/organisations',
					})
				).data;
				setOrganisations(organisationsResponse.data);
				setActiveOrganisation(organisationsResponse.data[0]);
			} catch (error) {
				return error;
			}
		};

		const fetchDashboardData = async () => {
			try {
				if (organisations.length === 0) await getOrganisations();
				if (!activeorganisation.uid) return;
				const dashboardResponse = await request({
					url: `/dashboard/${activeorganisation.uid}/summary?startDate=${filterDates[0].startDate.toISOString().split('.')[0]}&endDate=${filterDates[0].endDate.toISOString().split('.')[0]}&type=${
						filterFrequency || 'daily'
					}`,
				});
				setDashboardData(dashboardResponse.data.data);

				const chartData = dashboardResponse.data.data.message_data;
				const labels = [0, ...chartData.map((label) => label.data.date)];
				const dataSet = [0, ...chartData.map((label) => label.count)];
				const data = {
					labels,
					datasets: [
						{
							data: dataSet,
							fill: false,
							borderColor: '#477DB3',
							tension: 0.5,
							yAxisID: 'yAxis',
							xAxisID: 'xAxis',
						},
					],
				};

				if (!Chart.getChart('chart') || !Chart.getChart('chart')?.canvas) {
					new Chart(document.getElementById('chart'), { type: 'line', data, options });
				} else {
					const currentChart = Chart.getChart('chart');
					currentChart.data.labels = labels;
					currentChart.data.datasets[0].data = dataSet;
					currentChart.update();
				}
			} catch (error) {
				return error;
			}
		};

		fetchDashboardData();
		if (activeTab === 'apps') getApps({ page: 1 });
		if (activeTab === 'events') getEvents({ page: 1 });
	}, [options, activeTab, filterDates, activeorganisation, organisations, filterFrequency, getEvents, getApps]);

	return (
		<div className="dashboard">
			<header className="dashboard--header">
				<div className="dashboard--header--container">
					<div className="logo">
						<img src={ConvoyLogo} alt="convoy logo" />
					</div>

					<button className="user">
						<div>
							<div className="icon">O</div>
							<div className="name">{activeorganisation && activeorganisation.name}</div>
						</div>
						<img src={ArrowDownIcon} alt="arrow down icon" />
						<div className="dropdown organisations">
							<ul>
								<li onClick={() => logout()}>Logout</li>
							</ul>
						</div>
					</button>
				</div>
			</header>

			<div className="dashboard--page">
				<div className={`filter ${showFilterCalendar ? 'show-calendar' : ''}`}>
					Filter by:
					<button className="filter--button" onClick={() => toggleShowFilterCalendar(!showFilterCalendar)}>
						<img src={CalendarIcon} alt="calender icon" />
						<div>
							{getDate(filterDates[0].startDate)} - {getDate(filterDates[0].endDate)}
						</div>
						<img src={ArrowDownIcon} alt="arrow down icon" />
					</button>
					<DateRange onChange={(item) => setFilterDates([item.selection])} moveRangeOnFirstSelection={false} ranges={filterDates} />
					<select value={filterFrequency} onChange={(event) => setFilterFrequency(event.target.value)}>
						<option value="daily">Daily</option>
						<option value="weekly">Weekly</option>
						<option value="monthly">Monthly</option>
						<option value="yearly">Yearly</option>
					</select>
				</div>

				<div className="dashboard--page--details">
					<div className="card dashboard--page--details--chart">
						<ul>
							<li className="messages">
								<img src={MessageIcon} alt="message icon" />
								<div className="metric">
									<div>{dashboardData.messages_sent}</div>
									<div>{dashboardData.messages_sent === 1 ? 'Event' : 'Events'} Sent</div>
								</div>
							</li>
							<li className="apps">
								<img src={AppsIcon} alt="apps icon" />
								<div className="metric">
									<div>{dashboardData.apps}</div>
									<div>{dashboardData.apps === 1 ? 'App' : 'Apps'}</div>
								</div>
							</li>
						</ul>

						<div>
							<h3>Events Sent</h3>
							<canvas id="chart" width="400" height="200"></canvas>
						</div>
					</div>

					<div className="card has-title dashboard--page--details--credentials">
						<div className="card--title">
							<h2>Organization Details</h2>
						</div>

						<div className="card--container">
							<div className="auth-item">
								<div>
									<div className="auth-item--label">Organisation ID</div>
									<div className="auth-item--item">{activeorganisation.uid}</div>
								</div>
								<button className="copy" onClick={() => copyText(activeorganisation.uid)}>
									<img src={CopyIcon} alt="copy icon" />
								</button>
							</div>
						</div>

						<div className="card--footer">
							<button className="primary" onClick={() => (window.location = 'https://github.com/frain-dev/convoy')}>
								Go to docs
							</button>
						</div>
					</div>
				</div>

				<section className="card dashboard--logs">
					<div className="dashboard--logs--tabs">
						<div className="tabs">
							{tabs.map((tab, index) => (
								<button
									onClick={() => {
										setActiveTab(tab);
										setDetailsItem();
										setEventDeliveryAtempt({
											ip_address: '',
											http_status: '',
											api_version: '',
											updated_at: 0,
											deleted_at: 0,
										});
									}}
									key={index}
									className={'clear tab ' + (activeTab === tab ? 'active' : '')}
								>
									{tab}
								</button>
							))}
						</div>

						<div className="table">
							{activeTab && activeTab === 'events' && (
								<React.Fragment>
									<table>
										<thead>
											<tr className="table--head">
												<th scope="col">Status</th>
												<th scope="col">Event Type</th>
												<th scope="col">Attempts</th>
												<th scope="col">Next Retry</th>
												<th scope="col">Date Created</th>
												<th scope="col">Next Entry</th>
											</tr>
										</thead>
										<tbody>
											{events.content.map((event, index) => (
												<tr
													key={index}
													onClick={() => {
														setDetailsItem(event);
														getDelieveryAttempts(event.uid);
													}}
												>
													<td>
														<div>
															<div className="tag">{event.status}</div>
														</div>
													</td>
													<td>
														<div>{event.event_type}</div>
													</td>
													<td>
														<div>{event.metadata.num_trials}</div>
													</td>
													<td>
														<div>{getDate(event.metadata.next_send_time)}</div>
													</td>
													<td>
														<div>{getDate(event.created_at)}</div>
													</td>
													<td>
														<div>
															<button
																disabled={event.status === 'Success' || event.status === 'Scheduled'}
																className={'primary has-icon icon-left ' + (event.status === 'Success' || event.status === 'Scheduled' ? 'disable_action' : '')}
																onClick={() => retryEvent({ eventId: event.uid, appId: event.app_id })}
															>
																<img src={RefreshIcon} alt="refresh icon" /> Retry
															</button>
														</div>
													</td>
												</tr>
											))}
										</tbody>
									</table>

									{events.pagination.totalPage > 1 && (
										<div className="pagination">
											<button disabled={events.pagination.page === 1} onClick={() => getEvents({ page: events.pagination.page - 1 })} className="has-icon">
												<img src={AngleArrowLeftIcon} alt="angle icon left" />
											</button>
											<button disabled={events.pagination.page === events.pagination.totalPage} onClick={() => getEvents({ page: events.pagination.page + 1 })} className="has-icon">
												<img src={AngleArrowRightIcon} alt="angle icon right" />
											</button>
										</div>
									)}
								</React.Fragment>
							)}

							{activeTab && activeTab === 'apps' && (
								<React.Fragment>
									<table>
										<thead>
											<tr className="table--head">
												<th scope="col">Name</th>
												<th scope="col">Created</th>
												<th scope="col">Updated</th>
												<th scope="col">Number of Events</th>
												<th scope="col">Number of Endpoints</th>
											</tr>
										</thead>
										<tbody>
											{apps.content.map((app, index) => (
												<tr key={index} onClick={() => setDetailsItem(app)}>
													<td>
														<div>{app.name}</div>
													</td>
													<td>
														<div>{getDate(app.created_at)}</div>
													</td>
													<td>
														<div>{getDate(app.updated_at)}</div>
													</td>
													<td>
														<div>{app.events}</div>
													</td>
													<td>
														<div>{app.endpoints.length}</div>
													</td>
												</tr>
											))}
										</tbody>
									</table>

									{apps.pagination.totalPage > 1 && (
										<div className="pagination">
											<button disabled={apps.pagination.page === 1} onClick={() => getApps({ page: apps.pagination.page - 1 })} className="has-icon">
												<img src={AngleArrowLeftIcon} alt="angle icon left" />
											</button>
											<button disabled={apps.pagination.page === apps.pagination.totalPage} onClick={() => getApps({ page: apps.pagination.page + 1 })} className="has-icon">
												<img src={AngleArrowRightIcon} alt="angle icon right" />
											</button>
										</div>
									)}
								</React.Fragment>
							)}
						</div>
					</div>

					{detailsItem && (
						<div className="dashboard--logs--details">
							<h3>Details</h3>
							<ul className="dashboard--logs--details--meta">
								{eventDeliveryAtempt && eventDeliveryAtempt.ip_address && (
									<React.Fragment>
										<li>
											<div className="label">IP Address</div>
											<div className="value color">{eventDeliveryAtempt.ip_address}</div>
										</li>
										<li>
											<div className="label">HTTP Status</div>
											<div className="value">{eventDeliveryAtempt.http_status}</div>
										</li>
										<li>
											<div className="label">API Version</div>
											<div className="value color">{eventDeliveryAtempt.api_version}</div>
										</li>
									</React.Fragment>
								)}
								<li>
									<div className="label">Date Created</div>
									<div className="value">{getDate(detailsItem.created_at)}</div>
								</li>
								<li>
									<div className="label">Last Updated</div>
									<div className="value">{getDate(detailsItem.updated_at)}</div>
								</li>
							</ul>

							{activeTab === 'events' && (
								<React.Fragment>
									<h4>Event Data</h4>
									<div className={'dashboard--logs--details--event-data ' + (viewAllEventData && detailsItem.data ? '' : 'data-hidden')}>
										<ReactJson src={detailsItem.data} iconStyle="square" displayDataTypes={false} enableClipboard={false} style={jsonStyle} name={false} />
									</div>
									{detailsItem.data && (
										<div className="dashboard--logs--details--view-more">
											<button className="has-icon" onClick={() => toggleViewAllEventDataState(!viewAllEventData)}>
												<img src={AngleArrowDownIcon} alt="angle arrow down" />
												{viewAllEventData ? 'Hide more' : 'View more'}
											</button>
										</div>
									)}

									<hr />

									<h4>Response Data</h4>
									{eventDeliveryAtempt && (
										<div>
											<div className={'dashboard--logs--details--response-data ' + (viewAllResponseData && eventDeliveryAtempt.response_data ? '' : 'data-hidden')}>
												{eventDeliveryAtempt.response_data}
											</div>
											{eventDeliveryAtempt.response_data && eventDeliveryAtempt.response_data.length > 60 && (
												<div className="dashboard--logs--details--view-more">
													<button className="has-icon" onClick={() => toggleViewAllResponseData(!viewAllResponseData)}>
														{!viewAllResponseData && <img src={AngleArrowDownIcon} alt="angle arrow down" />}
														{viewAllResponseData && <img src={AngleArrowUpIcon} alt="angle arrow up" />}
														{viewAllResponseData ? 'Hide more' : 'View more'}
													</button>
												</div>
											)}
										</div>
									)}
								</React.Fragment>
							)}

							{activeTab === 'apps' && (
								<React.Fragment>
									<h4>App Event Endpoints</h4>
									<ul className="dashboard--logs--details--endpoints">
										{detailsItem.endpoints &&
											detailsItem.endpoints.map((endpoint, index) => (
												<li key={index}>
													<h5>{endpoint.description}</h5>
													<p>
														<img src={LinkIcon} alt="link icon" />
														{endpoint.target_url}
													</p>
												</li>
											))}
									</ul>
								</React.Fragment>
							)}
						</div>
					)}
				</section>
			</div>
		</div>
	);
}

export { DashboardPage };
