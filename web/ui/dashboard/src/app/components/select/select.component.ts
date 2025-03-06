import { CommonModule } from '@angular/common';
import { Component, ElementRef, EventEmitter, forwardRef, Input, OnInit, Output, ViewChild, AfterViewInit, OnChanges, SimpleChanges } from '@angular/core';
import { ControlContainer, ControlValueAccessor, NG_VALUE_ACCESSOR, ReactiveFormsModule } from '@angular/forms';
import { fromEvent } from 'rxjs';
import { debounceTime, distinctUntilChanged, map, startWith } from 'rxjs/operators';
import { ButtonComponent } from '../button/button.component';
import { DropdownComponent, DropdownOptionDirective } from '../dropdown/dropdown.component';
import { TooltipComponent } from '../tooltip/tooltip.component';
import { InputDirective, LabelComponent } from '../input/input.component';

@Component({
	selector: 'convoy-select',
	standalone: true,
	imports: [CommonModule, ReactiveFormsModule, TooltipComponent, DropdownComponent, ButtonComponent, DropdownOptionDirective, LabelComponent, InputDirective],
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
export class SelectComponent implements OnInit, OnChanges, AfterViewInit, ControlValueAccessor {
	@Input('options') options?: Array<any> = [];
	@Input('name') name!: string;
	@Input('errorMessage') errorMessage!: string;
	@Input('label') label!: string;
	@Input('formControlName') formControlName!: string;
	@Input('required') required = false;
	@Input('readonly') readonly = false;
	@Input('multiple') multiple = false;
	@Input('placeholder') placeholder!: string;
	@Input('className') class!: string;
	@Input('value') value!: any;
	@Input('tooltipContent') tooltipContent!: string;
	@Input('searchable') searchable: boolean = false;
	@Input('selectedValues') selectedValues: any = [];
	@Input('selectionType') selectionType: 'eventTypes' | 'default' = 'default';
	@Output('selectedOption') selectedOption = new EventEmitter<any>();
	@Output('searchString') searchString = new EventEmitter<any>();
	@ViewChild('searchFilter', { static: false }) searchFilter!: ElementRef;
	selectedValue: any;
	selectedOptions: any = [];

	control: any;
	private onChange: (value: any) => void = () => {};
	private onTouched: () => void = () => {};

	constructor(private controlContainer: ControlContainer) {}

	ngOnInit(): void {
		if (this.controlContainer.control?.get(this.formControlName)) {
			this.control = this.controlContainer.control.get(this.formControlName);

			// Initialize the selected value from control value if available
			if (this.control.value) {
				this.updateSelectedValueFromOptions(this.control.value);
			}
		}
	}

	ngOnChanges(changes: SimpleChanges): void {
		// Handle changes to the value input
		if (changes['value'] && changes['value'].currentValue) {
			this.updateSelectedValueFromOptions(changes['value'].currentValue);
		}

		// Handle changes to the options input
		if (changes['options'] && changes['options'].currentValue && this.value) {
			this.updateSelectedValueFromOptions(this.value);
		}
	}

	updateSelectedValueFromOptions(value: any): void {
		if (!this.options || this.options.length === 0) {
			return;
		}

		// For objects with name property (like event types)
		if (typeof this.options[0] === 'object' && 'name' in this.options[0]) {
			const option = this.options.find((opt: any) => opt.name === value || (typeof value === 'object' && value?.name === opt.name));

			if (option) {
				this.selectedValue = option;
			}
		}
		// For objects with uid property
		else if (typeof this.options[0] === 'object' && 'uid' in this.options[0]) {
			const option = this.options.find((opt: any) => opt.uid === value || (typeof value === 'object' && value?.uid === opt.uid));

			if (option) {
				this.selectedValue = option;
			}
		}
		// For simple string options
		else if (typeof this.options[0] === 'string') {
			const option = this.options.find((opt: string) => opt === value);

			if (option) {
				this.selectedValue = option;
			}
		}
	}

	selectOption(option: any) {
		this.selectedValue = option;
		this.onChange(option);
		this.onTouched();
		this.selectedOption.emit(option);
	}

	removeOption(option: any) {
		this.selectedOptions = this.selectedOptions.filter((e: any) => e !== option) || this.selectedOptions.filter((e: any) => e.uid !== option.uid) || this.selectedOptions.filter((e: any) => e.name !== option.name);
		this.updateSelectedOptions();
	}

	updateSelectedOptions() {
		if (!this.selectedOptions?.length) return;
		let selectedIds: any = [];

		this.selectedOptions.forEach((option: any) => {
			if (typeof option !== 'string') {
				if (this.selectionType === 'default') selectedIds.push(option.uid);
				else selectedIds.push(option.name);
			} else selectedIds.push(option);
		});

		this.selectedOption.emit(selectedIds);
	}

	get option(): string {
		if (this.selectedValue) {
			if (typeof this.selectedValue === 'string') {
				return this.selectedValue;
			} else if (typeof this.selectedValue === 'object') {
				return this.selectedValue.name || this.selectedValue.title || '';
			}
		}

		// Fall back to searching in options array
		if (this.options && this.value) {
			const found = this.options.find((item: any) => {
				if (typeof item === 'string') return item === this.value;
				if (typeof item === 'object' && 'uid' in item) return item.uid === this.value;
				if (typeof item === 'object' && 'name' in item) return item.name === this.value;
				return false;
			});

			if (found) {
				return typeof found === 'string' ? found : found.name || found.title || '';
			}
		}

		return '';
	}

	registerOnChange(fn: (value: any) => void): void {
		this.onChange = fn;
	}

	registerOnTouched(fn: () => void): void {
		this.onTouched = fn;
	}

	writeValue(value: string | Array<any>) {
		if (!value) return;

		if (this.multiple) {
			if (typeof value !== 'string' && this.selectedValues?.length) {
				this.selectedOptions = this.selectedValues;
			} else if (typeof value !== 'string' && this.selectedOptions?.length === 0 && this.selectedValues?.length === 0) {
				setTimeout(() => {
					if (Array.isArray(value)) {
						value.forEach((item: any) => {
							this.selectedOptions.push({
								uid: item,
								name: this.options?.find((option: any) => option.uid === item)?.name || this.options?.find((option: any) => option === item)
							});
						});
					}
				}, 100);
			}
		} else {
			// Update the selected value for single selection
			this.updateSelectedValueFromOptions(value);
		}
	}

	setDisabledState() {}

	ngAfterViewInit() {
		if (this.searchable && this.searchFilter) {
			fromEvent<any>(this.searchFilter.nativeElement, 'keyup')
				.pipe(
					map(event => event.target.value),
					startWith(''),
					debounceTime(500),
					distinctUntilChanged()
				)
				.subscribe(searchString => {
					this.searchString.emit(searchString);
				});
		}
	}
}
