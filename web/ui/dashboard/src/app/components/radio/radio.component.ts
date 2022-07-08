import { CommonModule } from '@angular/common';
import { Component, forwardRef, Input, OnInit } from '@angular/core';
import { ControlContainer, ControlValueAccessor, NG_VALUE_ACCESSOR, ReactiveFormsModule } from '@angular/forms';

@Component({
	selector: 'convoy-radio',
	standalone: true,
	imports: [CommonModule, ReactiveFormsModule],
	templateUrl: './radio.component.html',
	styleUrls: ['./radio.component.scss'],
	providers: [
		{
			provide: NG_VALUE_ACCESSOR,
			useExisting: forwardRef(() => RadioComponent),
			multi: true
		}
	]
})
export class RadioComponent implements OnInit, ControlValueAccessor {
	@Input('label') label!: string;
	@Input('_name') name!: string;
	@Input('value') value!: any;
	@Input('_id') id!: string;
	@Input('description') description!: string;
	@Input('formControlName') formControlName!: string;
	control!: any;

	constructor(private controlContainer: ControlContainer) {}

	ngOnInit(): void {
		if (this.controlContainer.control?.get(this.formControlName)) this.control = this.controlContainer.control.get(this.formControlName);
	}

	registerOnChange() {}

	registerOnTouched() {}

	writeValue() {}

	setDisabledState() {}
}
