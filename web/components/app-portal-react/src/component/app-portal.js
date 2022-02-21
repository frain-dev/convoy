import logo from '../logo.svg';
import './style.scss';

function AppPortal() {
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
						<tbody></tbody>
					</table>
				</div>
            </div>



            <section className="card dashboard--logs">
                <div className="dashboard--logs--tabs">
                    <div className="dashboard--logs--tabs--head tabs">
                        <div className="tabs">
                            {/*<button *ngFor="let tab of tabs" (click)="toggleActiveTab(tab)" className="clear tab" [ngClass]="{ active: activeTab === tab }">
                                {{ tab }}
                            </button>*/}
                        </div>
                    </div>

                    <div className="table">
                        {/*<ng-container *ngIf="activeTab === 'events'">*/}
                            <div className="filter">
                                <button
                                    className="filter--button date-filter-button"
                                >
                                    <img src="/assets/img/calendar-icon.svg" alt="calender icon" />
                                    {/*<mat-date-range-input [formGroup]="eventsFilterDateRange" [rangePicker]="eventsFilterPicker">
                                        <input matStartDate formControlName="startDate" placeholder="Start date" />
                                        <input matEndDate formControlName="endDate" placeholder="End date" (dateChange)="getEvents({ addToURL: true })" />
                                    </mat-date-range-input>
                        <mat-date-range-picker #eventsFilterPicker [disabled]="false"></mat-date-range-picker>*/}
                                    <img src="/assets/img/angle-arrow-down.svg" alt="arrow down icon" />
                                </button>

                                <button
                                    className="filter--button primary events-filter-clear-btn"
                                >
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
                                        {/*<ng-container *ngFor="let eventGroup of displayedEvents; let i = index">*/}
                                            <tr className="table--date-row">
                                                <td>
                                                    <div>Date</div>
                                                </td>
                                                <td></td>
                                                <td></td>
                                                <td></td>
                                            </tr>
                                            <tr>
                                                <td>
                                                    <div>
                                                        <div className="tag">Event TYpe</div>
                                                    </div>
                                                </td>
                                                <td className="has-long-text">
                                                    <div>Title</div>
                                                </td>
                                                <td>
                                                    <div>Date</div>
                                                </td>
                                                <td>
                                                    <div>
                                                        <button className="primary clear has-icon icon-right">
                                                            Deliveries
                                                            <img src="../../../../assets/img/angle-arrow-right-primary.svg" alt="arrow right" />
                                                        </button>
                                                    </div>
                                                </td>
                                            </tr>
                                        {/*</ng-container>*/}
                                    </tbody>
                                </table>

                                <div className="table--load-more button-container center">
                                    <button>
                                        <img src="/assets/img/arrow-down-icon.svg" alt="arrow down icon" />
                                        Load more
                                    </button>
                                </div>
                            </div>

                            <div className="empty-state table--container">
                                <img src="/assets/img/empty-state-img.svg" alt="empty state" />
                                <p>No event to show here</p>
                            </div>
                        {/*</ng-container>*/}

                        {/*<ng-container *ngIf="activeTab === 'event deliveries'">*/}
                            <div className="filter">
                                <button
                                    className="filter--button date-filter-button">
                                    <img src="/assets/img/calendar-icon.svg" alt="calender icon" />
                                    {/*<mat-date-range-input [formGroup]="eventDeliveriesFilterDateRange" [rangePicker]="eventDeliveriesFilterPicker">
                                        <input matStartDate formControlName="startDate" placeholder="Start date" />
                                        <input matEndDate formControlName="endDate" placeholder="End date" (dateChange)="getEventDeliveries({ addToURL: true })" />
                                    </mat-date-range-input>
                                    <mat-date-range-picker #eventDeliveriesFilterPicker [disabled]="false"></mat-date-range-picker>*/}
                                    <img src="/assets/img/angle-arrow-down.svg" alt="arrow down icon" />
                                </button>

                                <div className="dropdown">
                                    <button
                                        className="filter--button dropdown--button">
                                        <img src="/assets/img/status-filter-icon.svg" alt="status filter icon" />
                                        <span>Status</span>
                                        <img src="/assets/img/angle-arrow-down.svg" alt="arrow down icon" />
                                    </button>
                                    <div className="dropdown--list">
                                        <div className="dropdown--list--item">
                                            <input
                                                type="checkbox"
                                                name="status"/>
                                            <label>Status</label>
                                        </div>

                                        <button className="primary">Apply</button>
                                    </div>
                                </div>

                                <div className="filter--button event-button active">
                                    Event Filtered
                                    <button className="primary clear has-icon">
                                        <img src="../../../../assets/img/close-icon.svg" alt="close icon" />
                                    </button>
                                </div>

                                <button
                                    className="filter--button primary events-filter-clear-btn"
                                >
                                    Clear Filter
                                </button>
                            </div>

                            <hr />

                            <div className="table--actions button-container left">
                                <button className="primary clear has-icon icon-left hover">
                                    <img src="../../../../assets/img/refresh-icon-2.svg" alt="refresh icon" />
                                    Refresh
                                </button>
                                <button className="primary clear has-icon icon-left hover" >
                                    <img src="../../../../assets/img/retry-icon.svg" alt="retry icon" />
                                    Bulk Retry
                                </button>
                            </div>

                            <div className="table--container">
                                <table id="event-deliveries-table">
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
                                        <ng-container >
                                            <tr className="table--date-row">
                                                <td>
                                                    <div>
                                                        Date
                                                    </div>
                                                </td>
                                                <td></td>
                                                <td></td>
                                                <td></td>
                                                <td></td>
                                            </tr>
                                            <tr>
                                                <td>
                                                    <div className="checkbox has-retry">
                                                        <img src="/assets/img/retry-icon.svg" alt="retry icon" title="manually retried" />
                                                        <input type="checkbox" id="event" />
                                                        <div>Status</div>
                                                    </div>
                                                </td>
                                                <td>
                                                    <div>Name</div>
                                                </td>
                                                <td>
                                                    <div>Trials</div>
                                                </td>
                                                <td>
                                                    <div>Date</div>
                                                </td>
                                                <td>
                                                    <div>
                                                        <button>
                                                            <img src="/assets/img/refresh-icon.svg" alt="refresh icon" />
                                                            Retry
                                                        </button>
                                                    </div>
                                                </td>
                                            </tr>
                                        </ng-container>
                                    </tbody>
                                </table>

                                <div className="table--load-more button-container center">
                                    <button>
                                        <img src="/assets/img/arrow-down-icon.svg" alt="arrow down icon" />
                                        Load more
                                    </button>
                                </div>
                            </div>

                            <div className="empty-state table--container">
                                <img src="/assets/img/empty-state-img.svg" alt="empty state" />
                                <p>No event to show here</p>
                            </div>
                        </ng-container>
                    </div>
                </div>

                <div className="dashboard--logs--details">
                    {/*<ng-container *ngIf="detailsItem">*/}
                        <h3>Details</h3>
                        <ul className="dashboard--logs--details--meta" >
                            {/*<ng-container *ngIf="activeTab === 'event deliveries'">*/}
                                <li className="list-item-inline">
                                    <div className="list-item-inline--label">IP Address</div>
                                    <div className="list-item-inline--item color">IP</div>
                                </li>
                                <li className="list-item-inline">
                                    <div className="list-item-inline--label">HTTP Status</div>
                                    <div className="list-item-inline--item">Status</div>
                                </li>
                                <li className="list-item-inline">
                                    <div className="list-item-inline--label">API Version</div>
                                    <div className="list-item-inline--item color">Version</div>
                                </li>
                                <li className="list-item-inline">
                                    <div className="list-item-inline--label">Endpoint</div>
                                    <div className="list-item-inline--item color">URL</div>
                                </li>
                                <li className="list-item-inline">
                                    <div className="list-item-inline--label">Next Retry</div>
                                    <div className="list-item-inline--item color">Date</div>
                                </li>
                                <li className="list-item-inline">
                                    <div className="list-item-inline--label">App Name</div>
                                    <div className="list-item-inline--item color">Title</div>
                                </li>
                            {/*</ng-container>*/}
                        </ul>

                        <ul className="tabs">
                            <li>
                                <button className="primary outline">Label</button>
                            </li>
                        </ul>

                        <div className="dashboard--logs--details--req-res">
                            <div>
                                <h3>Event</h3>
                                <prism language="json" [code]="getCodeSnippetString(activeTab === 'events' ? 'event' : 'event_delivery')"></prism>
                            </div>

                            <div [className]="'dashboard--logs--details--tabs-data ' + (eventDetailsActiveTab === 'response' ? 'show' : '')">
                                <h3>Header</h3>
                                <prism language="json" [code]="getCodeSnippetString('res_head')"></prism>

                                <h3>Body</h3>
                                <prism language="json" [code]="getCodeSnippetString('res_body')"></prism>
                            </div>

                            <div [className]="'dashboard--logs--details--tabs-data ' + (eventDetailsActiveTab === 'request' ? 'show' : '')">
                                <h3>Header</h3>
                                <prism language="json" [code]="getCodeSnippetString('req')"></prism>
                            </div>
                        </div>

                        <ng-container *ngIf="activeTab === 'events'">
                            <h4>Deliveries Overview</h4>
                            <ul className="dashboard--logs--details--endpoints inline">
                                <li *ngFor="let delivery of sidebarEventDeliveries">
                                    <div [className]="'tag tag--' + delivery.status">
                                        {{ delivery.status }}
                                    </div>
                                    <div className="url" [title]="delivery.endpoint.target_url">
                                        {{ delivery.endpoint.target_url }}
                                    </div>
                                </li>
                            </ul>
                        </ng-container>
                    </ng-container>
                </div>
            </section>
		</div>
	);
}

export default AppPortal;
