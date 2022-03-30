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

describe('ConvoyDashboardComponent', () => {
	let component: ConvoyDashboardComponent;
	let fixture: ComponentFixture<ConvoyDashboardComponent>;

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

		expect(response.status).toBe(true);
		expect(response.data.length).toBeGreaterThanOrEqual(1);

		fixture.detectChanges();
		expect(component.groups.length).toBeGreaterThanOrEqual(1);
		expect(component.activeGroup).toBeTruthy();

		const groupDropdown: HTMLElement = fixture.debugElement.nativeElement.querySelector('#groups-dropdown');
		console.log('🚀 ~ file: convoy-dashboard.component.spec.ts ~ line 61 ~ it ~ groupDropdown', groupDropdown);
		expect(groupDropdown).toBeTruthy();
		expect(groupDropdown.children.length).toBeGreaterThanOrEqual(1);
	});

	// it('can handle groups UI', async () => {
	// 	expect(component.groups.length).toBeGreaterThanOrEqual(1);
	// });

	// it('can get groups from API', async () => {
	// 	const response = await component.getGroups();
	// 	console.log('🚀 ~ file: convoy-dashboard.component.spec.ts ~ line 48 ~ it ~ response', response.status);
	// 	expect(response.status).toBe(true);
	// 	expect(response.data.length >= 1).toBe(true);
	// });
});
