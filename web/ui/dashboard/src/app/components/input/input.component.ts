import { Component, Directive, ElementRef, Input, OnInit, Optional } from '@angular/core';

import { TooltipComponent } from '../tooltip/tooltip.component';

/* ================== Input directive ================== */
@Directive({
	selector: '[convoy-input]',
	standalone: true,
	host: {
		class: 'transition-colors duration-[.3s] w-full font-body font-normal text-[14px] text-new.text-primary placeholder:text-new.text-placeholder bg-white-100 border border-new.border rounded-8px shadow-[0px_2px_2px_-1px_rgba(0,0,0,0.04)] outline-none focus:border-new.primary-400 disabled:text-new.text-placeholder disabled:bg-new.surface-subtle py-10px px-12px appearance-none',
		'[class.pointer-events-none]': 'isReadonly',
		'[class.appearance-none]': "type !== 'password'"
	}
})
export class InputDirective implements OnInit {
	type!: string;
	isReadonly = false;
	showLoginPassword = false;

	constructor(private element: ElementRef) {}

	ngOnInit(): void {
		const el = this.element.nativeElement as HTMLInputElement;
		this.type = el.getAttribute('type') || '';
		this.isReadonly = el.hasAttribute('readonly');
	}
}

/* ================== Input field directive ================== */
@Directive({
	selector: 'convoy-input-field, [convoy-input-field]',
	standalone: true,
	host: {
		class: 'w-full relative mb-24px block',
		'[class]': 'class'
	}
})
export class InputFieldDirective {
	@Input('className') class!: string;
}

/* ================== Password input component ================== */
@Component({
    selector: 'convoy-password-field',
    imports: [],
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
    imports: [],
    template: `
		<div class="flex items-center font-body text-[12px] mt-8px">
			<svg width="16" height="16" class="mr-6px fill-error-9">
				<use xlink:href="#error-icon"></use>
			</svg>
			<span class="text-new.error-500"><ng-content></ng-content></span>
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
    imports: [TooltipComponent],
    host: {
        class: 'w-full mb-8px flex items-center'
    },
    template: `
		<div class="flex items-center font-body font-normal text-[14px] leading-normal text-new.text-secondary">
		  <ng-content></ng-content>
		  @if (required === 'true') {
		    <span class="ml-2px">*</span>
		  }
		  @if (tooltip) {
		    <convoy-tooltip class="ml-4px" size="sm">{{ tooltip }}</convoy-tooltip>
		  }
		</div>
		`,
    styleUrls: ['./input.component.scss']
})
export class LabelComponent implements OnInit {
	@Optional() @Input('tooltip') tooltip!: string;
	@Optional() @Input('required') required: 'false' | 'true' = 'false';

	constructor() {}

	ngOnInit(): void {}
}
