import { CommonModule, DatePipe } from '@angular/common';
import { HttpClient, HttpClientModule } from '@angular/common/http';
import { ElementRef } from '@angular/core';
import { ComponentFixture, fakeAsync, flush, TestBed, tick } from '@angular/core/testing';
import { FormBuilder, FormsModule, ReactiveFormsModule } from '@angular/forms';
import { MatNativeDateModule } from '@angular/material/core';
import { MatDatepickerModule } from '@angular/material/datepicker';
import { ActivatedRoute } from '@angular/router';
import { RouterTestingModule } from '@angular/router/testing';

import { ConvoyDashboardComponent } from './convoy-dashboard.component';
import { ConvoyDashboardService } from './convoy-dashboard.service';
import { ConvoyLoaderComponent } from './shared-components/loader.component';
import { ConvoyTableLoaderComponent } from './shared-components/table-loader.component';
import { PrismModule } from './prism/prism.module';
import { MetricPipe } from './shared/pipes';
import { SharedModule } from './shared/shared.module';
import Chart from 'chart.js/auto';
import { HTTP_RESPONSE } from './models/global.model';
import { ConvoyNotificationComponent } from './shared-components/notification.component';

describe('ConvoyDashboardComponent', () => {
	let component: ConvoyDashboardComponent;
	let fixture: ComponentFixture<ConvoyDashboardComponent>;
	let convoyDashboardService: ConvoyDashboardService;

	const fakeActivatedRoute = {
		snapshot: { data: {} }
	} as ActivatedRoute;

	beforeEach(async () => {
		await TestBed.configureTestingModule({
			imports: [HttpClientModule, SharedModule, RouterTestingModule, CommonModule, MatDatepickerModule, MatNativeDateModule, FormsModule, ReactiveFormsModule],
			declarations: [ConvoyDashboardComponent, ConvoyLoaderComponent, ConvoyTableLoaderComponent, ConvoyNotificationComponent],
			providers: [HttpClient, FormBuilder, DatePipe, { provide: ActivatedRoute, useValue: fakeActivatedRoute }, ConvoyDashboardService]
		}).compileComponents();
	});

	beforeEach(() => {
		fixture = TestBed.createComponent(ConvoyDashboardComponent);
		component = fixture.componentInstance;
		convoyDashboardService = TestBed.get(ConvoyDashboardService);

		component.requestToken = 'ZGVmYXVsdDpkZWZhdWx0';
		component.isCloud = false;
		component.groupId = '5c9c6db0-7606-4f9f-9965-5455980881a2';
		component.apiURL = 'http://localhost:5005/ui';

		fixture.detectChanges();
	});

	it('should create', () => {
		expect(component).toBeTruthy();
	});

	// groups UI has been rewritten
	// it('can get groups and render', fakeAsync(() => {
	// 	const groups: HTTP_RESPONSE = require('./mock/groups.json');
	// 	spyOn(convoyDashboardService, 'getGroups').and.returnValue(Promise.resolve(groups));
	// 	component.getGroups();

	// 	// confirm loaders are visible
	// 	const dashboardSummaryLoader: HTMLElement = fixture.debugElement.nativeElement.querySelector('#dashboard_summary_loader');
	// 	const groupConfigLoader: HTMLElement = fixture.debugElement.nativeElement.querySelector('#group_config_loader');
	// 	const eventsTableLoader: HTMLElement = fixture.debugElement.nativeElement.querySelector('#events_loader_loader');
	// 	const eventDeliveriesLoader: HTMLElement = fixture.debugElement.nativeElement.querySelector('#event_deliveries_loader');
	// 	const appsLoader: HTMLElement = fixture.debugElement.nativeElement.querySelector('#apps_loader');
	// 	const detailsSectionLoader: HTMLElement = fixture.debugElement.nativeElement.querySelector('#details_section_loader');
	// 	expect(dashboardSummaryLoader && groupConfigLoader && eventsTableLoader && eventDeliveriesLoader && appsLoader && detailsSectionLoader).toBeTruthy();

	// 	tick();
	// 	fixture.detectChanges();

	// 	// API response
	// 	expect(component.groups.length).toBeGreaterThanOrEqual(1);
	// 	// expect(convoyDashboardService.activeGroupId).toBeTruthy();

	// 	// UI implementation
	// 	const groupDropdown: HTMLElement = fixture.debugElement.nativeElement.querySelector('#groups-dropdown');
	// 	expect(groupDropdown).toBeTruthy();
	// 	expect(groupDropdown.children.length).toBeGreaterThanOrEqual(1);
	// }));

	it('can get group config and render', fakeAsync(() => {
		const groupConfig: HTTP_RESPONSE = require('./mock/config.json');
		spyOn(convoyDashboardService, 'getConfigDetails').and.returnValue(Promise.resolve(groupConfig));
		component.getConfigDetails();

		const groupConfigLoader: HTMLElement = fixture.debugElement.nativeElement.querySelector('#group_config_loader');
		expect(groupConfigLoader).toBeTruthy();

		tick();
		fixture.detectChanges();

		const groupConfigLoader2: HTMLElement = fixture.debugElement.nativeElement.querySelector('#group_config_loader');
		expect(groupConfigLoader2).toBeFalsy();

		const groupConfigContainer: HTMLElement = fixture.debugElement.nativeElement.querySelector('#group-config');
		expect(groupConfigContainer).toBeTruthy();
		const configUiItems = groupConfigContainer.querySelectorAll('.list-item-inline--item');
		configUiItems.forEach(element => {
			expect(element.textContent).toBeTruthy();
		});
		expect(configUiItems.length).toEqual(4);
	}));

	it('can get dashboard data and render', fakeAsync(async () => {
		const dashboardSummary: HTTP_RESPONSE = require('./mock/dashboard_summary.json');
		spyOn(convoyDashboardService, 'dashboardSummary').and.returnValue(Promise.resolve(dashboardSummary));
		const response = await component.fetchDashboardData();
		tick();
		fixture.detectChanges();

		// API response
		expect(response.status).toBe(true);
		expect(Object.keys(response.data)).toEqual(['events_sent', 'apps', 'period', 'event_data']);

		// UI implementation
		const metricsContainer: HTMLElement = fixture.debugElement.nativeElement.querySelector('.metrics');
		expect(metricsContainer).toBeTruthy();
		metricsContainer.querySelectorAll('.metric div:first-of-type').forEach(element => {
			expect(element.textContent).toBeTruthy();
		});

		// Chart implementation
		expect(Chart.getChart('dahboard_events_chart') || Chart.getChart('dahboard_events_chart')?.canvas).toBeTruthy();
	}));

	it('can get events and render', fakeAsync(async () => {
		const events: HTTP_RESPONSE = require('./mock/events.json');
		const eventDeliveries: HTTP_RESPONSE = require('./mock/event_deliveries_few.json');
		spyOn(convoyDashboardService, 'getEvents').and.returnValue(Promise.resolve(events));
		spyOn(convoyDashboardService, 'getEventDeliveries').and.returnValue(Promise.resolve(eventDeliveries));
		const response = await component.getEvents();
		await component.toggleActiveTab('events');
		tick();
		fixture.detectChanges();

		// API response
		expect(response.status).toBe(true);
		expect(typeof response.data.content).toEqual('object');
		expect(component.events).toBeTruthy();
		expect(component.displayedEvents).toBeTruthy();
		expect(component.eventsDetailsItem).toBeTruthy();
		expect(component.sidebarEventDeliveries).toBeTruthy();

		// UI implementation
		const eventsEmptyStateContainer: HTMLElement = fixture.debugElement.nativeElement.querySelector('#events-empty-state');
		expect(eventsEmptyStateContainer).toBeFalsy();

		const eventsTableContainer: HTMLElement = fixture.debugElement.nativeElement.querySelector('#events-table-container');
		expect(eventsTableContainer).toBeTruthy();
		expect(eventsTableContainer.querySelector('#table')).toBeTruthy();
		expect(eventsTableContainer.querySelectorAll('#table thead th').length).toEqual(4);
		expect(component.displayedEvents.length === eventsTableContainer.querySelectorAll('#table tbody .table--date-row').length).toBeTrue();
		component.displayedEvents.forEach((event, index) => {
			expect(event.content.length === eventsTableContainer.querySelectorAll('#table tbody tr#event' + index).length).toBeTrue();
		});
	}));

	it('can handle empty events and render empty state', fakeAsync(() => {
		const emptyResponse: HTTP_RESPONSE = require('./mock/empty_response.json');
		spyOn(convoyDashboardService, 'getEvents').and.returnValue(Promise.resolve(emptyResponse));
		component.getEvents();
		component.toggleActiveTab('events');
		tick();
		fixture.detectChanges();

		// API response
		expect(component.events.content.length).toEqual(0);
		expect(component.displayedEvents.length).toEqual(0);
		expect(component.eventsDetailsItem).toBeFalsy();
		expect(component.sidebarEventDeliveries).toBeFalsy();

		// UI implementation
		const eventsEmptyStateContainer: HTMLElement = fixture.debugElement.nativeElement.querySelector('#events-empty-state');
		expect(eventsEmptyStateContainer).toBeTruthy();
	}));

	it('can get apps and render', fakeAsync(async () => {
		const apps: HTTP_RESPONSE = require('./mock/apps.json');
		const appPortalKey: HTTP_RESPONSE = require('./mock/app_portal_key.json');
		spyOn(convoyDashboardService, 'getApps').and.returnValue(Promise.resolve(apps));
		spyOn(convoyDashboardService, 'getAppPortalToken').and.returnValue(Promise.resolve(appPortalKey));
		const response = await component.getApps({ type: 'apps' });
		await component.toggleActiveTab('apps');
		tick();
		fixture.detectChanges();

		// API response
		expect(response.status).toBe(true);
		expect(typeof response.data.content).toEqual('object');
		expect(component.apps).toBeTruthy();
		expect(component.displayedApps).toBeTruthy();
		expect(component.appsDetailsItem).toBeTruthy();
		expect(component.appPortalLink).toBeTruthy();
		expect(component.filteredApps).toBeTruthy();

		// UI implementation
		const appsTableContainer: HTMLElement = fixture.debugElement.nativeElement.querySelector('#apps-table-container');
		expect(appsTableContainer.hasAttribute('hidden')).toBeFalse();
		expect(appsTableContainer.querySelector('#table')).toBeTruthy();
		expect(appsTableContainer.querySelectorAll('#table thead th').length).toEqual(8);
		// expect(component.apps.content.length === appsTableContainer.querySelectorAll('#table tbody tr').length).toBeTrue();
		expect(component.displayedApps.length === appsTableContainer.querySelectorAll('#table tbody .table--date-row').length).toBeTrue();
		component.displayedApps.forEach((app, index) => {
			expect(app.content.length === appsTableContainer.querySelectorAll('#table tbody tr#app' + index).length).toBeTrue();
		});
		const appsEmptyStateContainer: HTMLElement = fixture.debugElement.nativeElement.querySelector('#apps-empty-state');
		expect(appsEmptyStateContainer).toBeFalsy();
	}));

	it('can handle empty apps and render empty state', fakeAsync(async () => {
		const emptyResponse: HTTP_RESPONSE = require('./mock/empty_response.json');
		spyOn(convoyDashboardService, 'getApps').and.returnValue(Promise.resolve(emptyResponse));
		const response = await component.getApps({ type: 'apps' });
		await component.toggleActiveTab('apps');
		tick();
		fixture.detectChanges();

		// API response
		expect(response.status).toBe(true);
		expect(component.apps.content.length).toEqual(0);
		expect(component.appsDetailsItem).toBeFalsy();
		expect(component.appPortalLink).toBeFalsy();
		expect(component.filteredApps.length).toEqual(0);

		// UI implementation
		const appsEmptyStateContainer: HTMLElement = fixture.debugElement.nativeElement.querySelector('#apps-empty-state');
		expect(appsEmptyStateContainer).toBeTruthy();
		const appsTableContainer: HTMLElement = fixture.debugElement.nativeElement.querySelector('#apps-table-container');
		expect(appsTableContainer.hasAttribute('hidden')).toBeTrue();
	}));

	it('can get event deliveries and render', fakeAsync(async () => {
		const eventDeliveries: HTTP_RESPONSE = require('./mock/event_deliveries.json');
		const eventDeliveryAttempt: HTTP_RESPONSE = require('./mock/delivery_attempts.json');
		spyOn(convoyDashboardService, 'getEventDeliveries').and.returnValue(Promise.resolve(eventDeliveries));
		spyOn(convoyDashboardService, 'getEventDeliveryAttempts').and.returnValue(Promise.resolve(eventDeliveryAttempt));
		const response = await component.getEventDeliveries();
		await component.toggleActiveTab('event deliveries');
		tick();
		fixture.detectChanges();

		// API response
		expect(response.status).toBe(true);
		expect(response.data.content.length).toBeGreaterThanOrEqual(1);
		expect(component.eventDeliveries.content.length).toBeGreaterThanOrEqual(1);
		expect(component.displayedEventDeliveries.length).toBeGreaterThanOrEqual(1);
		expect(component.eventDeliveryAtempt).toBeTruthy();

		// UI implementation
		const eventDeliveryTableContainer: HTMLElement = fixture.debugElement.nativeElement.querySelector('#event-deliveries-table-container');
		expect(eventDeliveryTableContainer.hasAttribute('hidden')).toBeFalsy();
		expect(eventDeliveryTableContainer.querySelector('#table')).toBeTruthy();
		expect(eventDeliveryTableContainer.querySelectorAll('#table thead th').length).toEqual(6);
		expect(component.displayedEventDeliveries.length === eventDeliveryTableContainer.querySelectorAll('#table tbody .table--date-row').length).toBeTrue();
		component.displayedEventDeliveries.forEach((event, index) => {
			expect(event.content.length === eventDeliveryTableContainer.querySelectorAll('#table tbody tr#eventDel' + index).length).toBeTrue();
		});
		const eventDeliveryEmptyContainer: HTMLElement = fixture.debugElement.nativeElement.querySelector('#event-deliveries-empty-state');
		expect(eventDeliveryEmptyContainer).toBeFalsy();
	}));

	it('can handle empty event deliveries and render empty state', fakeAsync(async () => {
		const emptyResponse: HTTP_RESPONSE = require('./mock/empty_response.json');
		spyOn(convoyDashboardService, 'getEventDeliveries').and.returnValue(Promise.resolve(emptyResponse));
		const response = await component.getEventDeliveries();
		await component.toggleActiveTab('event deliveries');
		tick();
		fixture.detectChanges();

		// API response
		expect(component.eventDeliveries.content.length).toEqual(0);
		expect(component.displayedEventDeliveries.length).toEqual(0);
		expect(component.eventDeliveryAtempt).toBeFalsy();
		expect(component.displayedEventDeliveries.length).toEqual(0);

		// UI implementation
		const eventDeliveryTableContainer: HTMLElement = fixture.debugElement.nativeElement.querySelector('#event-deliveries-table-container');
		expect(eventDeliveryTableContainer.hasAttribute('hidden')).toBeTruthy();
		const eventDeliveryEmptyContainer: HTMLElement = fixture.debugElement.nativeElement.querySelector('#event-deliveries-empty-state');
		expect(eventDeliveryEmptyContainer).toBeTruthy();
	}));

	it('can create app', fakeAsync(async () => {
		const apps: HTTP_RESPONSE = require('./mock/apps.json');
		const appPortalKey: HTTP_RESPONSE = require('./mock/app_portal_key.json');
		const createAppResponse: HTTP_RESPONSE = require('./mock/create_app.json');
		spyOn(convoyDashboardService, 'getApps').and.returnValue(Promise.resolve(apps));
		spyOn(convoyDashboardService, 'getAppPortalToken').and.returnValue(Promise.resolve(appPortalKey));
		spyOn(convoyDashboardService, 'createApp').and.returnValue(Promise.resolve(createAppResponse));
		await component.toggleActiveTab('apps');

		const createAppModalButton = fixture.debugElement.nativeElement.querySelector('#create-app-modal-button');
		expect(createAppModalButton).toBeTruthy();
		createAppModalButton.click();
		fixture.detectChanges();
		expect(fixture.debugElement.nativeElement.querySelector('#create-app-form')).toBeTruthy();
		expect(fixture.debugElement.nativeElement.querySelector('#create-app-button')).toBeTruthy();

		component.addNewAppForm.patchValue({ name: 'test-app', support_email: 'test@yopmail.com' });
		fixture.detectChanges();

		component.createNewApp();
		tick();
		fixture.detectChanges();
		flush();

		expect(fixture.debugElement.nativeElement.querySelector('#create-app-form')).toBeFalsy();
		expect(fixture.debugElement.nativeElement.querySelector('#create-app-button')).toBeFalsy();
	}));

	it('add endpoint to app', fakeAsync(async () => {
		const apps: HTTP_RESPONSE = require('./mock/apps.json');
		const appPortalKey: HTTP_RESPONSE = require('./mock/app_portal_key.json');
		const addEndpointResponse: HTTP_RESPONSE = require('./mock/add_endpoint.json');
		spyOn(convoyDashboardService, 'getApps').and.returnValue(Promise.resolve(apps));
		spyOn(convoyDashboardService, 'getAppPortalToken').and.returnValue(Promise.resolve(appPortalKey));
		spyOn(convoyDashboardService, 'addNewEndpoint').and.returnValue(Promise.resolve(addEndpointResponse));
		await component.toggleActiveTab('apps');

		component.addNewEndpointForm.patchValue({ url: 'https://webhook.site/989bdf00-6fa8-4d1f-9980-95b1e0912b94', description: 'test' });
		component.eventTags = ['test', 'test2'];
		fixture.detectChanges();

		component.addNewEndpoint();
		tick();
		fixture.detectChanges();
		flush();

		expect(component.showAddEndpointModal).toBeFalse();
		expect(component.isCreatingNewEndpoint).toBeFalse();
	}));

	it('send event', fakeAsync(async () => {
		const apps: HTTP_RESPONSE = require('./mock/apps.json');
		const appPortalKey: HTTP_RESPONSE = require('./mock/app_portal_key.json');
		const createEventResponse: HTTP_RESPONSE = require('./mock/create_event.json');
		spyOn(convoyDashboardService, 'getApps').and.returnValue(Promise.resolve(apps));
		spyOn(convoyDashboardService, 'getAppPortalToken').and.returnValue(Promise.resolve(appPortalKey));
		spyOn(convoyDashboardService, 'sendEvent').and.returnValue(Promise.resolve(createEventResponse));
		await component.toggleActiveTab('apps');

		component.sendEventForm.patchValue({ app_id: 'https', description: "{ test: 'test' }", event_type: ['test'] });
		fixture.detectChanges();

		component.sendNewEvent();
		tick();
		fixture.detectChanges();
		flush();

		expect(component.showAddEventModal).toBeFalse();
		expect(component.isSendingNewEvent).toBeFalse();
	}));
});
