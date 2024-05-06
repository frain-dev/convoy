import { Component, Input, OnInit } from '@angular/core';
import { CommonModule } from '@angular/common';
import { STATUS_COLOR } from 'src/app/models/global.model';

@Component({
	selector: 'convoy-tag, [convoy-tag]',
	standalone: true,
	imports: [CommonModule],
	template: `
		<ng-content></ng-content>
	`,
	host: { class: 'rounded-22px w-fit text-center text-12 justify-between gap-x-4px disabled:opacity-50', '[class]': 'classes' }
})
export class TagComponent implements OnInit {
	@Input('className') class!: string;

	@Input('fill') fill: 'outline' | 'soft' | 'solid' | 'soft-outline' = 'soft';
	@Input('color') color: 'primary' | 'error' | 'success' | 'warning' | 'neutral' = 'neutral';
	@Input('size') size: 'sm' | 'md' | 'lg' = 'md';

	fontSizes = { xs: 'text-10', sm: 'text-10', md: `text-12`, lg: `text-14` };
	spacing = { xs: 'px-8px py-0', sm: `py-2px px-6px`, md: `py-4px px-12px`, lg: `py-8px px-14px w-full` };

	constructor() {}

	ngOnInit(): void {}

	get classes() {
		const colors = {
			primary: { solid: 'bg-new.primary-400 text-white-100', outline: 'border border-new.primary-400 text-new.primary-400', soft: 'bg-new.primary-25 text-new.primary-400', 'soft-outline': 'border border-new.primary-200 bg-new.primary-25 text-new.primary-400' },
			error: { solid: 'bg-error-9 text-white-100', outline: 'border border-error-9 text-error-9', soft: 'bg-error-a3 text-error-11', 'soft-outline': 'border border-error-6 bg-error-a2 text-error-9' },
			neutral: { solid: 'bg-neutral-9 text-white-100', outline: 'border border-neutral-9 text-neutral-9', soft: 'bg-neutral-a3 text-neutral-11', 'soft-outline': 'border border-neutral-6 bg-neutral-a2 text-neutral-9' },
			success: { solid: 'bg-success-9 text-white-100', outline: 'border border-success-9 text-success-9', soft: 'bg-success-a3 text-success-11', 'soft-outline': 'border border-success-6 bg-success-a2 text-success-9' },
			warning: { solid: 'bg-warning-9 text-white-100', outline: 'border border-warning-9 text-warning-9', soft: 'bg-warning-a3 text-warning-11', 'soft-outline': 'border border-warning-6 bg-warning-a2 text-warning-9' }
		};

		return `rounded-22px ${this.fontSizes[this.size]} ${this.spacing[this.size]} ${colors[this.color][this.fill]}`;
	}
}
