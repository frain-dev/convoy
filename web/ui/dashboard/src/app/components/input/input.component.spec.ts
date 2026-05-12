import { Component } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { InputDirective } from './input.component';

@Component({
	standalone: true,
	imports: [InputDirective],
	template: `<input convoy-input type="text" />`
})
class InputHostComponent {}

describe('InputDirective', () => {
	beforeEach(async () => {
		await TestBed.configureTestingModule({
			imports: [InputHostComponent]
		}).compileComponents();
	});

	it('should create', () => {
		const fixture = TestBed.createComponent(InputHostComponent);
		expect(fixture).toBeTruthy();
	});
});
