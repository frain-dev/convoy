import { CommonModule, DatePipe } from '@angular/common';
import { HttpClient, HttpClientModule } from '@angular/common/http';
import { ElementRef } from '@angular/core';
import { ComponentFixture, TestBed } from '@angular/core/testing';
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

describe('ConvoyDashboardComponent', () => {
	let component: ConvoyDashboardComponent;
	let fixture: ComponentFixture<ConvoyDashboardComponent>;
	let activeG;

	const fakeActivatedRoute = {
		snapshot: { data: {} }
	} as ActivatedRoute;

	beforeEach(async () => {
		await TestBed.configureTestingModule({
			imports: [HttpClientModule, SharedModule, RouterTestingModule, CommonModule, MatDatepickerModule, MatNativeDateModule, FormsModule, ReactiveFormsModule],
			declarations: [ConvoyDashboardComponent, ConvoyLoaderComponent],
			providers: [HttpClient, FormBuilder, DatePipe, { provide: ActivatedRoute, useValue: fakeActivatedRoute }]
		}).compileComponents();
	});

	beforeEach(() => {
		fixture = TestBed.createComponent(ConvoyDashboardComponent);
		component = fixture.componentInstance;

		component.requestToken = 'ZGVmYXVsdDpkZWZhdWx0';
		component.isCloud = false;
		component.groupId = '5c9c6db0-7606-4f9f-9965-5455980881a2';
		component.apiURL = 'http://localhost:5005/ui';

		fixture.detectChanges();
	});

	it('should create', () => {
		expect(component).toBeTruthy();
	});

	it('can get groups and render', async () => {
		const response = await component.getGroups();
		fixture.detectChanges();

		// API response
		expect(response.status).toBe(true);
		expect(response.data.length).toBeGreaterThanOrEqual(1);
		expect(component.groups.length).toBeGreaterThanOrEqual(1);
		expect(component.activeGroup).toBeTruthy();

		// UI implementation
		const groupDropdown: HTMLElement = fixture.debugElement.nativeElement.querySelector('#groups-dropdown');
		expect(groupDropdown).toBeTruthy();
		expect(groupDropdown.children.length).toBeGreaterThanOrEqual(1);
	});

	it('can get group credentials and render', async () => {
		const response = await component.getConfigDetails();
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
	});

	it('can get dashboard data and render', async () => {
		const response = await component.fetchDashboardData();
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
	});

	it('can get events and render', async () => {
		const response = await component.getEvents();
		await component.toggleActiveTab('events');
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
	});

	it('can get apps and render', async () => {
		const response = await component.getApps({ type: 'apps' });
		await component.toggleActiveTab('apps');
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
	});

	it('can get event deliveries and render', async () => {
		const response = await component.getEventDeliveries();
		await component.toggleActiveTab('event deliveries');
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
	});
});
