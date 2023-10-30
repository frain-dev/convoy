import { ComponentFixture, TestBed } from '@angular/core/testing';

import { TableLoaderComponent } from './endpoints-filter.component';

describe('TableLoaderComponent', () => {
	let component: TableLoaderComponent;
	let fixture: ComponentFixture<TableLoaderComponent>;

	beforeEach(async () => {
		await TestBed.configureTestingModule({
			declarations: [TableLoaderComponent]
		}).compileComponents();
	});

	beforeEach(() => {
		fixture = TestBed.createComponent(TableLoaderComponent);
		component = fixture.componentInstance;
		fixture.detectChanges();
	});

	it('should create', () => {
		expect(component).toBeTruthy();
	});
});
