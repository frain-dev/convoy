import { CommonModule } from '@angular/common';
import { Component, EventEmitter, forwardRef, Input, OnInit, Output } from '@angular/core';
import { ControlContainer, ControlValueAccessor, NG_VALUE_ACCESSOR, ReactiveFormsModule } from '@angular/forms';
import { ButtonComponent } from '../button/button.component';
import { DropdownComponent, DropdownOptionDirective } from '../dropdown/dropdown.component';
import { TooltipComponent } from '../tooltip/tooltip.component';

@Component({
	selector: 'convoy-select',
	standalone: true,
	imports: [CommonModule, ReactiveFormsModule, TooltipComponent, DropdownComponent, ButtonComponent, DropdownOptionDirective],
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
	@Input('options') options?: Array<any> = [];
	@Input('name') name!: string;
	@Input('errorMessage') errorMessage!: string;
	@Input('label') label!: string;
	@Input('formControlName') formControlName!: string;
	@Input('required') required = false;
	@Input('multiple') multiple = false;
	@Input('placeholder') placeholder!: string;
	@Input('className') class!: string;
	@Input('value') value!: any;
	@Input('tooltipPosition') tooltipPosition: 'left' | 'right' = 'left';
	@Input('tooltipSize') tooltipSize: 'sm' | 'md' = 'md';
	@Input('tooltipContent') tooltipContent!: string;
	@Output('onChange') onChange = new EventEmitter<any>();
	@Output('selectedOption') selectedOption = new EventEmitter<any>();
	selectedValue: any;
	selectedOptions: any = [];

	control: any;

	constructor(private controlContainer: ControlContainer) {}

	ngOnInit(): void {
		if (this.controlContainer.control?.get(this.formControlName)) this.control = this.controlContainer.control.get(this.formControlName);
	}

	selectOption(option?: any) {
		if (this.multiple) {
			const selectOption = this.selectedOptions?.find((item: any) => item === option) || this.selectedOptions?.find((item: any) => item.uid === option);
			if (!selectOption) this.selectedOptions.push(option);
			this.updateSelectedOptions();
		} else this.selectedOption.emit(option?.uid || option);
	}

	removeOption(option: any) {
		this.selectedOptions = this.selectedOptions.filter((e: any) => e !== option) || this.selectedOptions.filter((e: any) => e.uid !== option.uid);
		this.updateSelectedOptions();
	}

	updateSelectedOptions() {
		const selectedIds = typeof this.selectedOptions[0] !== 'string' ? this.selectedOptions.map((item: any) => item.uid) : this.selectedOptions;
		this.selectedOption.emit(selectedIds);
	}

	get option(): string {
		return this.options?.find(item => item.uid === this.value)?.name || this.options?.find(item => item === this.value) || '';
	}

	registerOnChange() {}

	registerOnTouched() {}

	writeValue(value: string | Array<any>) {
		if (value) {
			if (this.options?.length && typeof this.options[0] !== 'string' && !this.multiple) return (this.selectedValue = this.options?.find(option => option.uid === value));
			if (this.multiple && typeof value !== 'string' && this.selectedOptions?.length === 0) {
				setTimeout(() => {
					value.forEach((item: any) => {
						this.selectedOptions.push({
							uid: item,
							name: this.options?.find(option => option.uid === item)?.name || this.options?.find(option => option === item)
						});
					});
				}, 100);
			}
			if (!this.multiple) return (this.selectedValue = value);
		}
	}

	setDisabledState() {}
}
