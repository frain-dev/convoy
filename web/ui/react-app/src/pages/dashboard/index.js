// temporarily unused
// import React, { useState, useEffect } from 'react';
// import * as axios from 'axios';
import React from 'react';
import ArrowDownIcon from '../../assets/img/arrow-down-icon.svg';
import AppsIcon from '../../assets/img/apps-icon.svg';
import MessageIcon from '../../assets/img/message-icon.svg';
import EndpointsIcon from '../../assets/img/endpoints-icon.svg';
import Chart from '../../assets/img/chart.svg';
import RefreshIcon from '../../assets/img/refresh-icon.svg';
import CopyIcon from '../../assets/img/copy-icon.svg';
import ViewIcon from '../../assets/img/view-icon.svg';
import './app.scss';

function DashboardPage() {
	// temporarily unused code

	// const request = axios.default;
	// const [dashboardData, setDashboardData] = useState({});

	// useEffect(() => {
	// 	const fetchDashboardData = async () => {
	// 		try {
	// 			const dashboardResponse = await request.get('http://192.168.1.170:5005/v1/dashboard/d1c71511-1dcb-49e9-9ae9-e8da4d5d3560/summary?startDate=2021-08-16T00:00:00&endDate=2021-08-16T21:59:55');
	// 			console.log('🚀 ~ file: index.js ~ line 19 ~ useEffect ~ dashboardResponse', dashboardResponse);
	// 		} catch (error) {
	// 			console.log('🚀 ~ file: index.js ~ line 24 ~ fetchDashboardData ~ error', error);
	// 		}
	// 	};
	// 	fetchDashboardData();
	// });

	return (
		<div className="dashboard">
			<header className="dashboard--header">
				<div className="dashboard--header--container">
					<div className="logo">Fhooks.</div>

					<div className="user">
						<div className="icon">O</div>
						<div className="name">Company Name</div>
						<img src={ArrowDownIcon} alt="arrow down icon" />
					</div>
				</div>
			</header>

			<div className="dashboard--page">
				<div className="filter">Filter by: </div>

				<div className="dashboard--page--details">
					<div className="card dashboard--page--details--chart">
						<ul>
							<li>
								<img src={MessageIcon} alt="message icon" />
								<div className="metric">
									<div>2,589</div>
									<div>Message Sent</div>
								</div>
							</li>
							<li>
								<img src={AppsIcon} alt="apps icon" />
								<div className="metric">
									<div>2,589</div>
									<div>Apps</div>
								</div>
							</li>
							<li>
								<img src={EndpointsIcon} alt="endpoints icon" />
								<div className="metric">
									<div>2,589</div>
									<div>Endpoints</div>
								</div>
							</li>
						</ul>

						<div>
							<h3>Message Sent</h3>

							<img src={Chart} alt="chart" />
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
							<button className="clear tab active">Messages</button>
							<button className="clear tab">Apps</button>
						</div>

						<div className="table">
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
						</div>
					</div>

					<div className="dashboard--logs--details">
						<h3>Details</h3>
						<ul>
							<li>
								<div className="label">Time</div>
								<div className="value">Aug 5, 2021 12:23PM</div>
							</li>
							<li>
								<div className="label">IP Address</div>
								<div className="value">54.123.246.72</div>
							</li>
							<li>
								<div className="label">API version</div>
								<div className="value color">2021-08-27</div>
							</li>
						</ul>
					</div>
				</section>
			</div>
		</div>
	);
}

export { DashboardPage };
