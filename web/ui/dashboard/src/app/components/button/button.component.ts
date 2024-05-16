import { ChangeDetectionStrategy, Component, Input, OnInit } from '@angular/core';
import { CommonModule } from '@angular/common';

@Component({
	selector: 'convoy-button, [convoy-button]',
	standalone: true,
	host: { class: 'flex items-center justify-center disabled:opacity-50 cursor-pointer', '[class]': 'classes' },
	imports: [CommonModule],
	templateUrl: './button.component.html',
	styleUrls: ['./button.component.scss'],
	changeDetection: ChangeDetectionStrategy.OnPush
})
export class ButtonComponent implements OnInit {
	@Input('buttonText') buttonText!: string;
	@Input('fill') fill: 'default' | 'outline' | 'clear' | 'text' | 'link' | 'soft' | 'soft-outline' = 'default';
	@Input('size') size: 'xs' | 'sm' | 'md' | 'lg' = 'md';
	@Input('color') color: 'primary' | 'success' | 'warning' | 'neutral' | 'error' = 'primary';

	@Input('index') tabIndex = 0;
	buttonSizes = { xs: 'py-4px px-8px text-12', sm: `py-6px px-16px text-12`, md: `py-8px px-18px text-12`, lg: `py-10px px-36px w-full text-14` };

	constructor() {}

	ngOnInit(): void {}

	get classes(): string {
		const colors = {
			primary: {
				default: 'bg-new.primary-400 text-white-100',
				outline: 'border border-new.primary-400 text-new.primary-400',
				clear: 'border-none text-new.primary-400',
				text: `border-none text-new.primary-400 ${this.size == 'sm' || this.size == 'xs' ? 'text-12' : ''}`,
				link: `border-none text-new.primary-400 underline decoration-new.primary-400 ${this.size == 'sm' || this.size == 'xs' ? 'text-12' : ''}`,
				soft: 'bg-new.primary-25 text-new.primary-400',
				'soft-outline': `border border-new.primary-200 text-new.primary-400`
			},
			error: {
				default: 'bg-error-9 text-white-100',
				outline: 'border border-error-9 text-error-9',
				clear: 'border-none text-error-9',
				text: `border-none text-error-9 ${this.size == 'sm' || this.size == 'xs' ? 'text-12' : ''}`,
				link: `border-none text-error-9 underline decoration-error-9 ${this.size == 'sm' || this.size == 'xs' ? 'text-12' : ''}`,
				soft: 'bg-error-a3 text-error-11',
				'soft-outline': `border border-error-6 text-error-9`
			},
			neutral: {
				default: 'bg-neutral-9 text-white-100',
				outline: 'border border-neutral-9 text-neutral-9',
				clear: 'border-none text-neutral-9',
				text: `border-none text-neutral-10 ${this.size == 'sm' || this.size == 'xs' ? 'text-12' : ''}`,
				link: `border-none text-neutral-10 underline decoration-neutral-10 ${this.size == 'sm' || this.size == 'xs' ? 'text-12' : ''}`,
				soft: 'bg-neutral-a3 text-neutral-11',
				'soft-outline': 'border border-neutral-6 text-neutral-9'
			},
			success: {
				default: 'bg-success-9 text-white-100',
				outline: 'border border-success-9 text-success-9',
				clear: 'border-none text-success-9',
				text: `border-none text-success-9 ${this.size == 'sm' || this.size == 'xs' ? 'text-12' : ''}`,
				link: `border-none text-success-9 underline decoration-success-9 ${this.size == 'sm' || this.size == 'xs' ? 'text-12' : ''}`,
				soft: 'bg-success-a3 text-success-11',
				'soft-outline': 'border border-success-6 text-success-9'
			},
			warning: {
				default: 'bg-warning-9 text-white-100',
				outline: 'border border-warning-9 text-warning-9',
				clear: 'border-none text-warning-9',
				text: `border-none text-warning-9 ${this.size == 'sm' || this.size == 'xs' ? 'text-12' : ''}`,
				link: `border-none text-warning-9 underline decoration-warning-9 ${this.size == 'sm' || this.size == 'xs' ? 'text-12' : ''}`,
				soft: 'bg-warning-a3 text-warning-11',
				'soft-outline': 'border border-warning-6 text-warning-9'
			}
		};



		return `${this.fill !== 'text' ? this.buttonSizes[this.size] : ''} ${colors[this.color][this.fill]} flex items-center justify-center disabled:opacity-50 rounded-8px`;
	}
}
