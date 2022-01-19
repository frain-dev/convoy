import { ComponentFixture, TestBed } from '@angular/core/testing';

import { PrismComponent } from './prism.component';

describe('SharedComponent', () => {
	let component: PrismComponent;
	let fixture: ComponentFixture<PrismComponent>;

	beforeEach(async () => {
		await TestBed.configureTestingModule({
			declarations: [PrismComponent]
		}).compileComponents();
	});

	beforeEach(() => {
		fixture = TestBed.createComponent(PrismComponent);
		component = fixture.componentInstance;
		fixture.detectChanges();
	});

	it('should create', () => {
		expect(component).toBeTruthy();
	});
});
