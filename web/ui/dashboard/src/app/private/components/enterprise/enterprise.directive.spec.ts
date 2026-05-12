import { Component } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { EnterpriseDirective } from './enterprise.directive';

@Component({
	standalone: true,
	imports: [EnterpriseDirective],
	template: `<ng-template convoy-enterprise><span>enterprise</span></ng-template>`
})
class EnterpriseHostComponent {}

describe('EnterpriseDirective', () => {
	beforeEach(async () => {
		await TestBed.configureTestingModule({
			imports: [EnterpriseHostComponent]
		}).compileComponents();
	});

	it('should create', () => {
		const fixture = TestBed.createComponent(EnterpriseHostComponent);
		expect(fixture).toBeTruthy();
	});
});
