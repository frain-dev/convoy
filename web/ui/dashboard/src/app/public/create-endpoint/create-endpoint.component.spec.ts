import { ComponentFixture, TestBed } from '@angular/core/testing';

import { CreatePortalEndpointComponent } from './create-endpoint.component';

describe('CreatePortalEndpointComponent', () => {
	let component: CreatePortalEndpointComponent;
	let fixture: ComponentFixture<CreatePortalEndpointComponent>;

	beforeEach(async () => {
		await TestBed.configureTestingModule({
			imports: [CreatePortalEndpointComponent]
		}).compileComponents();

		fixture = TestBed.createComponent(CreatePortalEndpointComponent);
		component = fixture.componentInstance;
		fixture.detectChanges();
	});

	it('should create', () => {
		expect(component).toBeTruthy();
	});
});
