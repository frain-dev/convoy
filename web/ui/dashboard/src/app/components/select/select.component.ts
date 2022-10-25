import { CommonModule } from '@angular/common';
import { Component, EventEmitter, forwardRef, Input, OnInit, Output, ViewChild } from '@angular/core';
import { ControlContainer, ControlValueAccessor, FormControlDirective, NG_VALUE_ACCESSOR, ReactiveFormsModule } from '@angular/forms';
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
	@Input('options') options?: Array<any>;
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
	control!: any;

	constructor(private controlContainer: ControlContainer) {}

	ngOnInit(): void {
		if (this.controlContainer.control?.get(this.formControlName)) this.control = this.controlContainer.control.get(this.formControlName);
	}

	selectOption(option: string) {
		this.value = option;
		this.selectedOption.emit(option);
	}

	get option(): string {
		return this.options?.find(item => item.uid === this.value)?.name || this.options?.find(item => item === this.value) || '';
	}

	registerOnChange() {}

	registerOnTouched() {}

	writeValue() {}

	setDisabledState() {}
}
