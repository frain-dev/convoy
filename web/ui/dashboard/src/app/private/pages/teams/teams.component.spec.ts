import { ComponentFixture, TestBed } from '@angular/core/testing';
import { FormsModule, ReactiveFormsModule } from '@angular/forms';
import { RouterTestingModule } from '@angular/router/testing';

import { TeamsComponent } from './teams.component';

describe('TeamsComponent', () => {
	let component: TeamsComponent;
	let fixture: ComponentFixture<TeamsComponent>;

	beforeEach(async () => {
		await TestBed.configureTestingModule({
			declarations: [TeamsComponent],
			imports: [RouterTestingModule, ReactiveFormsModule, FormsModule]
		}).compileComponents();
	});

	beforeEach(() => {
		fixture = TestBed.createComponent(TeamsComponent);
		component = fixture.componentInstance;
		fixture.detectChanges();
	});

	it('should create', () => {
		expect(component).toBeTruthy();
	});
});
