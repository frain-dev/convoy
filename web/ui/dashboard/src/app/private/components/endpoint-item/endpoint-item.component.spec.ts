import { ComponentFixture, TestBed } from '@angular/core/testing';

import { EndpointComponent } from './endpoint-item.component';

describe('EndpointFilterComponent', () => {
	let component: EndpointComponent;
	let fixture: ComponentFixture<EndpointComponent>;

	beforeEach(async () => {
		await TestBed.configureTestingModule({
			declarations: [EndpointComponent]
		}).compileComponents();
	});

	beforeEach(() => {
		fixture = TestBed.createComponent(EndpointComponent);
		component = fixture.componentInstance;
		fixture.detectChanges();
	});

	it('should create', () => {
		expect(component).toBeTruthy();
	});
});
