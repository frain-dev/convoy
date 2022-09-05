import { ComponentFixture, TestBed } from '@angular/core/testing';
import { ReactiveFormsModule } from '@angular/forms';
import { RouterTestingModule } from '@angular/router/testing';
import { InputComponent } from 'src/app/components/input/input.component';

import { CreateOrganisationComponent } from './create-organisation.component';

describe('CreateOrganisationComponent', () => {
	let component: CreateOrganisationComponent;
	let fixture: ComponentFixture<CreateOrganisationComponent>;

	beforeEach(async () => {
		await TestBed.configureTestingModule({
			declarations: [CreateOrganisationComponent],
			imports: [RouterTestingModule, ReactiveFormsModule, InputComponent]
		}).compileComponents();
	});

	beforeEach(() => {
		fixture = TestBed.createComponent(CreateOrganisationComponent);
		component = fixture.componentInstance;
		fixture.detectChanges();
	});

	it('should create', () => {
		expect(component).toBeTruthy();
	});
});
