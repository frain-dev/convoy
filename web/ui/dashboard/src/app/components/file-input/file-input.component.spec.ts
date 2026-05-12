import { ComponentFixture, TestBed } from '@angular/core/testing';

import { FileInputComponent } from './file-input.component';
import { RouterTestingModule } from '@angular/router/testing';

describe('FileInputComponent', () => {
	let component: FileInputComponent;
	let fixture: ComponentFixture<FileInputComponent>;

	beforeEach(async () => {
		await TestBed.configureTestingModule({
			imports: [RouterTestingModule, FileInputComponent]
		}).compileComponents();

		fixture = TestBed.createComponent(FileInputComponent);
		component = fixture.componentInstance;
		fixture.detectChanges();
	});

	it('should create', () => {
		expect(component).toBeTruthy();
	});
});
