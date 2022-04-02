import { CommonModule, DatePipe } from '@angular/common';
import { HttpClient, HttpClientModule } from '@angular/common/http';
import { ElementRef } from '@angular/core';
import { ComponentFixture, fakeAsync, TestBed, tick } from '@angular/core/testing';
import { FormBuilder, FormsModule, ReactiveFormsModule } from '@angular/forms';
import { MatNativeDateModule } from '@angular/material/core';
import { MatDatepickerModule } from '@angular/material/datepicker';
import { ActivatedRoute } from '@angular/router';
import { RouterTestingModule } from '@angular/router/testing';

import { ConvoyDashboardComponent } from './convoy-dashboard.component';
import { ConvoyDashboardService } from './convoy-dashboard.service';
import { ConvoyLoaderComponent } from './loader-component/loader.component';
import { PrismModule } from './prism/prism.module';
import { MetricPipe } from './shared/pipes';
import { SharedModule } from './shared/shared.module';
import Chart from 'chart.js/auto';
import { HTTP_RESPONSE } from './models/http.model';

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
			declarations: [ConvoyDashboardComponent, ConvoyLoaderComponent],
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

	it('can get groups and render', fakeAsync(() => {
		const groups: HTTP_RESPONSE = require('./mock/groups.json');
		spyOn(convoyDashboardService, 'getGroups').and.returnValue(Promise.resolve(groups));
		component.getGroups();
		tick();
		fixture.detectChanges();

		// API response
		expect(component.groups.length).toBeGreaterThanOrEqual(1);
		expect(convoyDashboardService.activeGroupId).toBeTruthy();

		// UI implementation
		const groupDropdown: HTMLElement = fixture.debugElement.nativeElement.querySelector('#groups-dropdown');
		expect(groupDropdown).toBeTruthy();
		expect(groupDropdown.children.length).toBeGreaterThanOrEqual(1);
	}));

	it('can get group config and render', fakeAsync(async () => {
		const groupConfig: HTTP_RESPONSE = require('./mock/config.json');
		spyOn(convoyDashboardService, 'getConfigDetails').and.returnValue(Promise.resolve(groupConfig));
		const response = await component.getConfigDetails();
		tick();
		fixture.detectChanges();

		expect(response.status).toBe(true);
		expect(Object.keys(response.data)).toEqual(['strategy', 'signature']);

		const groupConfigContainer: HTMLElement = fixture.debugElement.nativeElement.querySelector('#group-config');
		expect(groupConfigContainer).toBeTruthy();
		const configUiItems = groupConfigContainer.querySelectorAll('.list-item--item');
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
		if (component.displayedEvents.length > 0) {
			const eventsTableContainer: HTMLElement = fixture.debugElement.nativeElement.querySelector('#events-table-container');
			expect(eventsTableContainer).toBeTruthy();
			expect(eventsTableContainer.querySelector('#table')).toBeTruthy();
			expect(eventsTableContainer.querySelectorAll('#table thead th').length).toEqual(4);
			expect(component.displayedEvents.length === eventsTableContainer.querySelectorAll('#table tbody .table--date-row').length).toBeTrue();
			component.displayedEvents.forEach((event, index) => {
				expect(event.events.length === eventsTableContainer.querySelectorAll('#table tbody tr#event' + index).length).toBeTrue();
			});
		}
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
		expect(component.appsDetailsItem).toBeTruthy();
		expect(component.appPortalLink).toBeTruthy();
		expect(component.filteredApps).toBeTruthy();

		// UI implementation
		if (component.apps.content.length > 0) {
			const appsTableContainer: HTMLElement = fixture.debugElement.nativeElement.querySelector('#apps-table-container');
			expect(appsTableContainer).toBeTruthy();
			expect(appsTableContainer.querySelector('#table')).toBeTruthy();
			expect(appsTableContainer.querySelectorAll('#table thead th').length).toEqual(8);
			expect(component.apps.content.length === appsTableContainer.querySelectorAll('#table tbody tr').length).toBeTrue();
		}
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
		expect(typeof response.data.content).toEqual('object');
		expect(component.eventDeliveries).toBeTruthy();
		expect(component.displayedEventDeliveries).toBeTruthy();
		expect(component.eventDeliveryAtempt).toBeTruthy();

		// UI implementation
		if (component.displayedEventDeliveries.length > 0) {
			const eventDeliveryTableContainer: HTMLElement = fixture.debugElement.nativeElement.querySelector('#event-deliveries-table-container');
			expect(eventDeliveryTableContainer).toBeTruthy();
			expect(eventDeliveryTableContainer.querySelector('#table')).toBeTruthy();
			expect(eventDeliveryTableContainer.querySelectorAll('#table thead th').length).toEqual(5);
			expect(component.displayedEventDeliveries.length === eventDeliveryTableContainer.querySelectorAll('#table tbody .table--date-row').length).toBeTrue();
			component.displayedEventDeliveries.forEach((event, index) => {
				expect(event.events.length === eventDeliveryTableContainer.querySelectorAll('#table tbody tr#eventDel' + index).length).toBeTrue();
			});
		}
	}));
});
