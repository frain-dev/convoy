import { ComponentFixture, TestBed } from '@angular/core/testing';
import { ReactiveFormsModule } from '@angular/forms';
import { RouterTestingModule } from '@angular/router/testing';
import { InputComponent } from 'src/app/components/input/input.component';
import { RadioComponent } from 'src/app/components/radio/radio.component';
import { SelectComponent } from 'src/app/components/select/select.component';

import { CreateProjectComponent } from './create-project-component.component';

describe('CreateProjectComponent', () => {
	let component: CreateProjectComponent;
	let fixture: ComponentFixture<CreateProjectComponent>;

	beforeEach(async () => {
		await TestBed.configureTestingModule({
			declarations: [CreateProjectComponent],
			imports: [ReactiveFormsModule, RouterTestingModule, InputComponent, RadioComponent, SelectComponent]
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
