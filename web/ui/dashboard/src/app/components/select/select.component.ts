import { CommonModule } from '@angular/common';
import { Component, EventEmitter, forwardRef, Input, OnInit, Output, ViewChild } from '@angular/core';
import { ControlContainer, ControlValueAccessor, NG_VALUE_ACCESSOR, ReactiveFormsModule } from '@angular/forms';
import { ButtonComponent } from '../button/button.component';
import { DropdownComponent } from '../dropdown/dropdown.component';
import { TooltipComponent } from '../tooltip/tooltip.component';

@Component({
	selector: 'convoy-select',
	standalone: true,
	imports: [CommonModule, ReactiveFormsModule, TooltipComponent, DropdownComponent, ButtonComponent],
	templateUrl: './select.component.html',
	styleUrls: ['./select.component.scss'],
	providers: [
		{
			provide: NG_VALUE_ACCESSOR,
			useExisting: forwardRef(() => SelectComponent),
			multi: true
		}
	]
})
export class SelectComponent implements OnInit, ControlValueAccessor {
	@ViewChild(DropdownComponent) dropdownComponent!: DropdownComponent;
	@Input('options') options?: Array<any> = [];
	@Input('name') name!: string;
	@Input('errorMessage') errorMessage!: string;
	@Input('label') label!: string;
	@Input('formControlName') formControlName!: string;
	@Input('required') required = false;
	@Input('placeholder') placeholder!: string;
	@Input('className') class!: string;
	@Input('value') value!: any;
	@Input('tooltipPosition') tooltipPosition: 'left' | 'right' = 'left';
	@Input('tooltipSize') tooltipSize: 'sm' | 'md' = 'md';
	@Input('tooltipContent') tooltipContent!: string;
	@Output('onChange') onChange = new EventEmitter<any>();
	@Output('selectedOption') selectedOption = new EventEmitter<any>();
	selectedValue: any;

	control: any;

	constructor(private controlContainer: ControlContainer) {}

	ngOnInit(): void {
		if (this.controlContainer.control?.get(this.formControlName)) this.control = this.controlContainer.control.get(this.formControlName);
	}

	selectOption(option?: any) {
		this.selectedOption.emit(option?.uid || option);
	}

	get option(): string {
		return this.options?.find(item => item.uid === this.value)?.name || this.options?.find(item => item === this.value) || '';
	}

	registerOnChange() {}

	registerOnTouched() {}

	writeValue(value: string) {
		if (value) {
			if (this.options?.length && typeof this.options[0] !== 'string') return (this.selectedValue = this.options?.find(option => option.uid === value));
			return (this.selectedValue = value);
		}
	}

	setDisabledState() {}
}
