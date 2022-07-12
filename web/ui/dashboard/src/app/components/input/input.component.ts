import { Component, forwardRef, Input, OnInit } from '@angular/core';
import { CommonModule } from '@angular/common';
import { ControlContainer, ControlValueAccessor, NG_VALUE_ACCESSOR, ReactiveFormsModule } from '@angular/forms';
import { TooltipComponent } from '../tooltip/tooltip.component';

@Component({
	selector: 'convoy-input',
	standalone: true,
	imports: [CommonModule, ReactiveFormsModule, TooltipComponent],
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
	@Input('type') type = 'text';
	@Input('autocomplete') autocomplete!: string;
	@Input('errorMessage') errorMessage!: string;
	@Input('label') label!: string;
	@Input('formControlName') formControlName!: string;
	@Input('required') required = false;
	@Input('readonly') readonly = false;
	@Input('placeholder') placeholder!: string;
	@Input('tooltipPosition') tooltipPosition!: string;
	@Input('tooltipSize') tooltipSize!: string;
	@Input('tooltipContent') tooltipContent!: string;
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
