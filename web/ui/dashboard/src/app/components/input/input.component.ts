import { Component, Directive, ElementRef, forwardRef, Input, OnInit, Optional } from '@angular/core';
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
	@Input('type') type: 'text' | 'password' | 'number' | 'url' | 'email' = 'text';
	@Input('autocomplete') autocomplete!: string;
	@Input('errorMessage') errorMessage!: string;
	@Input('label') label!: string;
	@Input('formControlName') formControlName!: string;
	@Input('required') required = false;
	@Input('readonly') readonly = false;
	@Input('placeholder') placeholder!: string;
	@Input('className') class!: string;
	@Input('tooltipPosition') tooltipPosition: 'left' | 'right' = 'left';
	@Input('tooltipSize') tooltipSize: 'sm' | 'md' = 'md';
	@Input('tooltipContent') tooltipContent!: string;
	@Input('tooltipImg') tooltipImg!: string;
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

/* ================== Input directive ================== */
@Directive({
	selector: '[convoy-input]',
	standalone: true,
	host: {
		class: 'transition-all duration-[.3s] w-full font-light text-14 placeholder:text-grey-40 text-grey-100 border border-primary-500 valid:border-primary-500 disabled:border-primary-500 disabled:bg-[#F7F9FC] hover:bg-primary-500 hover:border-grey-20 focus:border-primary-100 focus:bg-white-100 outline-none rounded-4px placeholder:opacity-[.48] bg-[#F7F9FC] py-12px px-16px appearance-none',
		'[ngClass]': "{ 'pointer-events-none': readonly, 'appearance-none': type !== 'password' }"
	}
})
export class InputDirective implements OnInit {
	type!: string;
	showLoginPassword = false;

	constructor(private element: ElementRef) {}

	ngOnInit(): void {
		this.type = this.element.nativeElement.getAttribute('type');
	}
}

/* ================== Input field directive ================== */
@Directive({
	selector: 'convoy-input-field, [convoy-input-field]',
	standalone: true,
	host: {
		class: 'w-full relative mb-24px block'
	}
})
export class InputFieldDirective {}

/* ================== Password input component ================== */
@Component({
	selector: 'convoy-password-field',
	standalone: true,
	imports: [CommonModule],
	template: `
		<div class="w-full relative">
			<ng-content></ng-content>
		</div>
	`
})
export class PasswordInputFieldComponent implements OnInit {
	ngOnInit(): void {}
}

/* ================== Input error component ================== */
@Component({
	selector: 'convoy-input-error',
	standalone: true,
	imports: [CommonModule],
	template: `
		<div class="flex items-center text-12 text-danger-100 mt-8px">
			<img src="assets/img/input-error-icon.svg" class="mr-6px w-16px" alt="input error icon" />
			<span><ng-content></ng-content></span>
		</div>
	`
})
export class InputErrorComponent implements OnInit {
	constructor() {}

	ngOnInit(): void {}
}

/* ================== Input label component ================== */
@Component({
	selector: 'convoy-label, [convoy-label]',
	standalone: true,
	imports: [CommonModule, TooltipComponent],
	host: {
		class: 'w-full font-medium text-12 text-grey-40 mb-8px flex items-center justify-between'
	},
	template: `
		<div class="flex items-center">
			<ng-content></ng-content>
			<convoy-tooltip *ngIf="tooltip" class="ml-4px" size="sm">{{ tooltip }}</convoy-tooltip>
		</div>
		<span *ngIf="required === 'true'" class="text-10 bg-grey-10 rounded-4px px-1 font-normal">required</span>
	`,
	styleUrls: ['./input.component.scss'],
	providers: [
		{
			provide: NG_VALUE_ACCESSOR,
			useExisting: forwardRef(() => LabelComponent),
			multi: true
		}
	]
})
export class LabelComponent implements OnInit {
	@Optional() @Input('tooltip') tooltip!: string;
	@Optional() @Input('required') required: 'false' | 'true' = 'false';

	constructor() {}

	ngOnInit(): void {}
}
