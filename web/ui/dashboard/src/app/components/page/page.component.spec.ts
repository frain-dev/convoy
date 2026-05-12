import { Component } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { PageDirective } from './page.component';

@Component({
	standalone: true,
	imports: [PageDirective],
	template: `<div convoy-page size="md"></div>`
})
class PageHostComponent {}

describe('PageDirective', () => {
	beforeEach(async () => {
		await TestBed.configureTestingModule({
			imports: [PageHostComponent]
		}).compileComponents();
	});

	it('should create', () => {
		const fixture = TestBed.createComponent(PageHostComponent);
		expect(fixture).toBeTruthy();
	});
});
