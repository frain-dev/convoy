import { ComponentFixture, TestBed } from '@angular/core/testing';

import { SourceURLComponent } from './source-url.component';
import { RouterTestingModule } from '@angular/router/testing';

describe('SourceURLComponent', () => {
	let component: SourceURLComponent;
	let fixture: ComponentFixture<SourceURLComponent>;

	beforeEach(async () => {
		await TestBed.configureTestingModule({
			imports: [RouterTestingModule, SourceURLComponent]
		}).compileComponents();

		fixture = TestBed.createComponent(SourceURLComponent);
		component = fixture.componentInstance;
		fixture.detectChanges();
	});

	it('should create', () => {
		expect(component).toBeTruthy();
	});
});
