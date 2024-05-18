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
	@Input('fill') fill: 'solid' | 'outline' | 'text' | 'link' | 'soft' | 'soft-outline' = 'solid';
	@Input('size') size: 'xs' | 'sm' | 'md' | 'lg' = 'md';
	@Input('color') color: 'primary' | 'success' | 'warning' | 'neutral' | 'error' = 'primary';

	@Input('index') tabIndex = 0;
	buttonSizes = { xs: 'py-4px px-8px text-12', sm: `py-6px px-16px text-12`, md: `py-8px px-18px text-12`, lg: `py-10px px-36px w-full text-14` };

	constructor() {}

	ngOnInit(): void {}

	get classes(): string {
		const buttonTypes = {
			solid: `${this.color === 'primary' ? 'bg-new.primary-400 text-white-100' : `bg-${this.color}-9 text-white-100`}`,
			outline: `border ${this.color === 'primary' ? 'border-new.primary-400 text-new.primary-400' : `border-${this.color}-9 text-${this.color}-9`}`,
			text: `border-none ${this.color === 'primary' ? 'text-new.primary-400 ' : `text-${this.color}-${this.color === 'neutral' ? '10' : '9'}`} ${this.size == 'sm' || this.size == 'xs' ? 'text-12' : ''}`,
			link: `border-none ${this.color === 'primary' ? 'text-new.primary-400 decoration-new.primary-400' : `text-${this.color}-${this.color === 'neutral' ? '10' : '9'} decoration-${this.color}-${this.color === 'neutral' ? '10' : '9'}`} underline ${
				this.size == 'sm' || this.size == 'xs' ? 'text-12' : ''
			}`,
			soft: `${this.color === 'primary' ? 'bg-new.primary-25 text-new.primary-400' : `bg-${this.color}-a3 text-${this.color}-11`} `,
			'soft-outline': `border ${this.color === 'primary' ? 'border-new.primary-200 text-new.primary-400' : `border-${this.color}-6 text-${this.color}-9`}`
		};

		return `${this.fill !== 'text' ? this.buttonSizes[this.size] : ''} ${buttonTypes[this.fill]} flex items-center justify-center disabled:opacity-50 rounded-8px`;
	}
}
