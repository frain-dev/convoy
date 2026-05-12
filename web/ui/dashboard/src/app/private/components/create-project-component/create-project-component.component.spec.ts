import { ComponentFixture, TestBed } from '@angular/core/testing';

import { CreateProjectComponent } from './create-project-component.component';
import { RouterTestingModule } from '@angular/router/testing';

describe('CreateProjectComponent', () => {
	let component: CreateProjectComponent;
	let fixture: ComponentFixture<CreateProjectComponent>;

	beforeEach(async () => {
		await TestBed.configureTestingModule({
			imports: [ RouterTestingModule, CreateProjectComponent ]
		}).compileComponents();
	});

	beforeEach(() => {
		fixture = TestBed.createComponent(CreateProjectComponent);
		component = fixture.componentInstance;
		fixture.detectChanges();
	});

	it('should create', () => {
		expect(component).toBeTruthy();
	});
});
