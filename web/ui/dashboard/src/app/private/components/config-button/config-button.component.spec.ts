import { ComponentFixture, TestBed } from '@angular/core/testing';

import { ConfigButtonComponent } from './config-button.component';
import { RouterTestingModule } from '@angular/router/testing';

describe('ConfigButtonComponent', () => {
	let component: ConfigButtonComponent;
	let fixture: ComponentFixture<ConfigButtonComponent>;

	beforeEach(async () => {
		await TestBed.configureTestingModule({
			imports: [ RouterTestingModule, ConfigButtonComponent ]
		}).compileComponents();
	});

	beforeEach(() => {
		fixture = TestBed.createComponent(ConfigButtonComponent);
		component = fixture.componentInstance;
		fixture.detectChanges();
	});

	it('should create', () => {
		expect(component).toBeTruthy();
	});
});
