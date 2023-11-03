import { ComponentFixture, TestBed } from '@angular/core/testing';

import { EndpointFilterComponent } from './endpoints-filter.component';

describe('EndpointFilterComponent', () => {
	let component: EndpointFilterComponent;
	let fixture: ComponentFixture<EndpointFilterComponent>;

	beforeEach(async () => {
		await TestBed.configureTestingModule({
			declarations: [EndpointFilterComponent]
		}).compileComponents();
	});

	beforeEach(() => {
		fixture = TestBed.createComponent(EndpointFilterComponent);
		component = fixture.componentInstance;
		fixture.detectChanges();
	});

	it('should create', () => {
		expect(component).toBeTruthy();
	});
});
