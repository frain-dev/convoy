import React, { useEffect, useState } from 'react';
import * as axios from 'axios';
import ArrowDownIcon from '../../assets/img/arrow-down-icon.svg';
import AppsIcon from '../../assets/img/apps-icon.svg';
import MessageIcon from '../../assets/img/message-icon.svg';
import RefreshIcon from '../../assets/img/refresh-icon.svg';
import CalendarIcon from '../../assets/img/calendar-icon.svg';
import CopyIcon from '../../assets/img/copy-icon.svg';
import ViewIcon from '../../assets/img/view-icon.svg';
import Chart from 'chart.js/auto';
import { DateRange } from 'react-date-range';
import ReactJson from 'react-json-view';
import './app.scss';
import 'react-date-range/dist/styles.css';
import 'react-date-range/dist/theme/default.css';

const _axios = axios.default;
const request = _axios.create({ baseURL: 'http://localhost:5005/v1' });
const months = ['January', 'February', 'March', 'April', 'May', 'June', 'July', 'August', 'September', 'October', 'November', 'December'];

function DashboardPage() {
	const [dashboardData, setDashboardData] = useState({ apps: 0, messages: 0, messageData: [] });
	const [apps, setAppsData] = useState([]);
	const [tabs] = useState(['messages', 'apps']);
	const [activeTab, setActiveTab] = useState('apps');
	const [showFilterCalendar, toggleShowFilterCalendar] = useState(false);
	const [organisations, setOrganisations] = useState([]);
	const [activeorganisation, setActiveOrganisation] = useState({
		uid: '',
		name: '',
		created_at: 0,
		updated_at: 0,
		deleted_at: 0,
	});
	const [detailsItem, setDetailsItem] = useState();
	const [filterDates, setFilterDates] = useState([
		{
			startDate: new Date(),
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

	const [data] = useState();

	const getDate = (date) => {
		const _date = new Date(date);
		const day = _date.getDate();
		const month = _date.getMonth();
		const year = _date.getFullYear();
		return `${day} ${months[month]}, ${year}`;
	};

	useEffect(() => {
		const getApps = async () => {
			try {
				const appsResponse = await request.get('/apps');
				setAppsData(appsResponse.data.applications);
			} catch (error) {
				return error;
			}
		};

		const getOrganisations = async () => {
			try {
				const organisationsResponse = await request.get('/organisations');
				setOrganisations(organisationsResponse.data.organisations);
				setActiveOrganisation(organisationsResponse.data.organisations[0]);
				return organisationsResponse.data.organisations[0];
			} catch (error) {
				return error;
			}
		};

		const fetchDashboardData = async (uid) => {
			try {
				const dashboardResponse = await request.get(
					`/dashboard/${uid}/summary?type=daily&startDate=${filterDates[0].startDate.toISOString().split('.')[0]}&endDate=${filterDates[0].endDate.toISOString().split('.')[0]}`,
				);
				setDashboardData(dashboardResponse.data.dashboard);

				const chartData = dashboardResponse.data.dashboard.messageData;
				const labels = chartData.map((label) => label.day);
				const dataSet = chartData.map((label) => label.count);
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
				const chart = new Chart(document.getElementById('chart'), { type: 'line', data, options });
				chart.update();
			} catch (error) {
				console.log(error);
				return error;
			}
		};

		getOrganisations().then((organisation) => fetchDashboardData(organisation.uid));
		if (activeTab === 'apps') getApps();
	}, [options, activeTab, filterDates, data]);

	return (
		<div className="dashboard">
			<header className="dashboard--header">
				<div className="dashboard--header--container">
					<div className="logo">Fhooks.</div>

					<button className="user">
						<div>
							<div className="icon">O</div>
							<div className="name">{activeorganisation.name}</div>
						</div>
						<img src={ArrowDownIcon} alt="arrow down icon" />
						<div className="dropdown organisations">
							<ul>
								{organisations.map((organisation, index) => (
									<li key={index}>{organisation.name}</li>
								))}
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
				</div>

				<div className="dashboard--page--details">
					<div className="card dashboard--page--details--chart">
						<ul>
							<li>
								<img src={MessageIcon} alt="message icon" />
								<div className="metric">
									<div>{dashboardData.messages}</div>
									<div>Messages Sent</div>
								</div>
							</li>
							<li>
								<img src={AppsIcon} alt="apps icon" />
								<div className="metric">
									<div>{dashboardData.apps}</div>
									<div>Apps</div>
								</div>
							</li>
						</ul>

						<div>
							<h3>Message Sent</h3>

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
									<div className="auth-item--label">Username</div>
									<div className="auth-item--item">Company usersname</div>
								</div>
								<div className="copy">
									<img src={CopyIcon} alt="copy icon" />
								</div>
							</div>
							<div className="auth-item">
								<div>
									<div className="auth-item--label">Password</div>
									<div className="auth-item--item">********</div>
								</div>
								<div className="copy">
									<img src={ViewIcon} alt="view icon" />
								</div>
							</div>
						</div>

						<div className="card--footer">
							<p>Our documentation contains the libraries, API and SDKs you need to integrate Fhooks on your platform.</p>
							<button className="primary">Go to docs</button>
						</div>
					</div>
				</div>

				<section className="card dashboard--logs">
					<div className="dashboard--logs--tabs">
						<div className="tabs">
							{tabs.map((tab, index) => (
								<button onClick={() => setActiveTab(tab)} key={index} className={'clear tab ' + (activeTab === tab ? 'active' : '')}>
									{tab}
								</button>
							))}
						</div>

						<div className="table">
							{activeTab && activeTab === 'messages' && (
								<table>
									<thead>
										<tr className="table--head">
											<th scope="col">Status</th>
											<th scope="col">Event Type</th>
											<th scope="col">Event ID</th>
											<th scope="col">Created</th>
											<th scope="col">Next Entry</th>
										</tr>
									</thead>
									<tbody>
										<tr>
											<td>
												<div>
													<div className="tag">200 OK</div>
												</div>
											</td>
											<td>
												<div>customer.created</div>
											</td>
											<td>
												<div>evt-136776hjfy76734uh5j</div>
											</td>
											<td>
												<div>3 Aug,2021</div>
											</td>
											<td>
												<div>
													<button className="primary has-icon icon-left">
														<img src={RefreshIcon} alt="refresh icon" /> Resend
													</button>
												</div>
											</td>
										</tr>
										<tr>
											<td>
												<div>
													<div className="tag">200 OK</div>
												</div>
											</td>
											<td>
												<div>customer.created</div>
											</td>
											<td>
												<div>evt-136776hjfy76734uh5j</div>
											</td>
											<td>
												<div>3 Aug,2021</div>
											</td>
											<td>
												<div>
													<button className="primary has-icon icon-left">
														<img src={RefreshIcon} alt="refresh icon" /> Resend
													</button>
												</div>
											</td>
										</tr>
										<tr>
											<td>
												<div>
													<div className="tag">200 OK</div>
												</div>
											</td>
											<td>
												<div>customer.created</div>
											</td>
											<td>
												<div>evt-136776hjfy76734uh5j</div>
											</td>
											<td>
												<div>3 Aug,2021</div>
											</td>
											<td>
												<div>
													<button className="primary has-icon icon-left">
														<img src={RefreshIcon} alt="refresh icon" /> Resend
													</button>
												</div>
											</td>
										</tr>
										<tr>
											<td>
												<div>
													<div className="tag">200 OK</div>
												</div>
											</td>
											<td>
												<div>customer.created</div>
											</td>
											<td>
												<div>evt-136776hjfy76734uh5j</div>
											</td>
											<td>
												<div>3 Aug,2021</div>
											</td>
											<td>
												<div>
													<button className="primary has-icon icon-left">
														<img src={RefreshIcon} alt="refresh icon" /> Resend
													</button>
												</div>
											</td>
										</tr>
										<tr>
											<td>
												<div>
													<div className="tag">200 OK</div>
												</div>
											</td>
											<td>
												<div>customer.created</div>
											</td>
											<td>
												<div>evt-136776hjfy76734uh5j</div>
											</td>
											<td>
												<div>3 Aug,2021</div>
											</td>
											<td>
												<div>
													<button className="primary has-icon icon-left">
														<img src={RefreshIcon} alt="refresh icon" /> Resend
													</button>
												</div>
											</td>
										</tr>
										<tr>
											<td>
												<div>
													<div className="tag">200 OK</div>
												</div>
											</td>
											<td>
												<div>customer.created</div>
											</td>
											<td>
												<div>evt-136776hjfy76734uh5j</div>
											</td>
											<td>
												<div>3 Aug,2021</div>
											</td>
											<td>
												<div>
													<button className="primary has-icon icon-left">
														<img src={RefreshIcon} alt="refresh icon" /> Resend
													</button>
												</div>
											</td>
										</tr>
									</tbody>
								</table>
							)}

							{activeTab && activeTab === 'apps' && (
								<table>
									<thead>
										<tr className="table--head">
											<th scope="col">Name</th>
											<th scope="col">Created</th>
											<th scope="col">Updated</th>
										</tr>
									</thead>
									<tbody>
										{apps.map((app, index) => (
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
											</tr>
										))}
									</tbody>
								</table>
							)}
						</div>
					</div>

					{detailsItem && (
						<div className="dashboard--logs--details">
							<h3>Details</h3>
							<ul>
								<li>
									<div className="label">Date Created</div>
									<div className="value">{getDate(detailsItem.created_at)}</div>
								</li>
								<li>
									<div className="label">Last Updated</div>
									<div className="value">{getDate(detailsItem.updated_at)}</div>
								</li>
							</ul>

							{detailsItem.endpoints.length > 0 && (
								<React.Fragment>
									<h4>App Endpoints</h4>
									<div>
										<ReactJson src={detailsItem.endpoints} iconStyle="square" displayDataTypes={false} enableClipboard={false} style={jsonStyle} />
									</div>
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
