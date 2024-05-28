import { CommonModule } from '@angular/common';
import { Component, forwardRef, Input, OnInit } from '@angular/core';
import { ControlContainer, ControlValueAccessor, NG_VALUE_ACCESSOR, ReactiveFormsModule } from '@angular/forms';
import { TooltipComponent } from '../tooltip/tooltip.component';

@Component({
	selector: 'convoy-radio',
	standalone: true,
	imports: [CommonModule, ReactiveFormsModule, TooltipComponent],
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
	@Input('checked') checked = false;
	@Input('description') description!: string;
	@Input('tooltipPosition') tooltipPosition: 'left' | 'right' | 'top' | 'bottom' | 'top-right' | 'top-left' = 'top-left';
	@Input('tooltipSize') tooltipSize: 'sm' | 'md' = 'md';
	@Input('tooltipContent') tooltipContent!: string;
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
