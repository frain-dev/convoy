import {CommonModule} from '@angular/common';
import {
    AfterViewChecked,
    Component,
    ElementRef,
    EventEmitter,
    forwardRef,
    Input,
    OnInit,
    Output,
    ViewChild
} from '@angular/core';
import {ControlContainer, ControlValueAccessor, NG_VALUE_ACCESSOR, ReactiveFormsModule} from '@angular/forms';
import {fromEvent} from 'rxjs';
import {debounceTime, distinctUntilChanged, map, startWith} from 'rxjs/operators';
import {ButtonComponent} from '../button/button.component';
import {DropdownComponent, DropdownOptionDirective} from '../dropdown/dropdown.component';
import {InputDirective, LabelComponent} from '../input/input.component';

@Component({
	selector: 'convoy-select',
	standalone: true,
    imports: [CommonModule, ReactiveFormsModule, DropdownComponent, ButtonComponent, DropdownOptionDirective, LabelComponent, InputDirective],
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
export class SelectComponent implements OnInit, AfterViewChecked, ControlValueAccessor {
	private _options?: Array<any> = [];
	@Input('options')
	set options(value: Array<any> | undefined) {
		this._options = value;
		// When options are set, try to initialize selectedValue if control has a value
		if (this.control?.value && this._options?.length) {
			this.initializeSelectedValue();
		}
	}
	get options(): Array<any> | undefined {
		return this._options;
	}
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
	@ViewChild('dropdownRef', { static: false }) dropdownRef!: DropdownComponent;
	selectedValue: any;
	selectedOptions: any = [];

	control: any;

	constructor(private controlContainer: ControlContainer) {}

	ngOnInit(): void {
		if (this.controlContainer.control?.get(this.formControlName)) {
			this.control = this.controlContainer.control.get(this.formControlName);
			this.initializeSelectedValue();
		}
	}

	ngAfterViewChecked(): void {
		// Check if we need to initialize the selected value
		// This handles cases where the component becomes visible after being hidden
		if (this.control?.value && this.options?.length && !this.selectedValue) {
			this.initializeSelectedValue();
		}
	}

	private initializeSelectedValue(): void {
		const currentValue = this.control?.value;
		if (currentValue && this.options?.length) {
			const found = this.options.find(option => option.uid === currentValue || option === currentValue);
			if (found) {
				this.selectedValue = found;
			}
		}
	}

	selectOption(option?: any) {
		if (this.multiple) {
			const selectedOption =
				this.selectedOptions?.find((item: any) => item === option) ||
				this.selectedOptions?.find((item: any) => item.uid === option) ||
				this.selectedOptions?.find((item: any) => item.uid === option.uid) ||
				this.selectedOptions?.find((item: any) => item.name === option.name && item.uid === option.uid);
			if (!selectedOption) this.selectedOptions?.push(option);

			this.updateSelectedOptions();
		} else {
			this.selectedValue = option;
			this.selectedOption.emit(option?.uid || option);
			// Update form control value
			if (this.control) {
				this.control.setValue(option?.uid || option);
			}
			// Close dropdown
			if (this.dropdownRef) {
				this.dropdownRef.show = false;
			}
		}
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
		return this.options?.find(item => item.uid === this.value)?.name || this.options?.find(item => item.uid === this.value)?.title || this.options?.find(item => item === this.value) || '';
	}

	registerOnChange() {}

	registerOnTouched() {}

	writeValue(value: string | Array<any>) {
		if (value) {
			if (this.options?.length && typeof this.options[0] !== 'string' && !this.multiple) return (this.selectedValue = this.options?.find(option => option.uid === value));
			if (this.multiple && typeof value !== 'string' && this.selectedValues?.length) this.selectedOptions = this.selectedValues;
			if (this.multiple && typeof value !== 'string' && this.selectedOptions?.length === 0 && this.selectedValues?.length === 0) {
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

			this.selectedValues = [];
		}
	}

	setDisabledState() {}

	ngAfterViewInit() {
		if (this.searchable) {
			fromEvent<any>(this.searchFilter?.nativeElement, 'keyup')
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
