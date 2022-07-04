import { Component, forwardRef, Input, OnInit } from '@angular/core';
import { CommonModule } from '@angular/common';
import { AbstractControl, ControlContainer, ControlValueAccessor, FormControl, NG_VALUE_ACCESSOR, ReactiveFormsModule } from '@angular/forms';

@Component({
	selector: 'convoy-input',
	standalone: true,
	imports: [CommonModule, ReactiveFormsModule],
	templateUrl: './input.component.html',
	styleUrls: ['./input.component.scss'],
	providers: [
		{
			provide: NG_VALUE_ACCESSOR,
			useExisting: forwardRef(() => InputComponent),
			multi: true
		}
	]
})
export class InputComponent implements OnInit, ControlValueAccessor {
	@Input('name') name!: string;
	@Input('type') type!: string;
	@Input('autocomplete') autocomplete!: string;
	@Input('errorMessage') errorMessage!: string;
	@Input('label') label!: string;
	@Input('formControlName') formControlName!: string;
	@Input('required') required = false;
	@Input('readonly') readonly = false;
	@Input('placeholder') placeholder!: string;
	control!: any;
	showLoginPassword = false;

	constructor(private controlContainer: ControlContainer) {}

	ngOnInit(): void {
		if (this.controlContainer.control?.get(this.formControlName)) this.control = this.controlContainer.control.get(this.formControlName);
	}

	registerOnChange() {}

	registerOnTouched() {}

	writeValue() {}

	setDisabledState() {}
}
