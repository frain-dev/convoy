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
	@Input('fill') fill: 'text' | 'outline' | 'softOutline' | 'tab' | 'link' | 'solid' | 'soft' = 'solid';
	// @Input('fill') fill: 'solid' | 'outline' | 'clear' | 'text' | 'link' | 'soft' = 'solid';
	@Input('size') size: 'xs' | 'sm' | 'md' | 'lg' = 'md';
	@Input('color') color: 'primary' | 'success' | 'warning' | 'danger' | 'gray' | 'transparent' = 'primary';
	@Input('texture') texture: 'deep' | 'light' = 'deep';
	@Input('index') tabIndex = 0;
	buttonSizes = { xs: 'py-4px px-8px text-10', sm: `py-8px px-10px text-12`, md: `py-12px px-16px text-14`, lg: `py-16px px-36px w-full text-16` };

	constructor() {}

	ngOnInit(): void {}

	get classes(): string {
		const softColorLevels = { primary: '25', gray: '100', danger: '50', success: '50', warning: '50', transparent: '25' };

		const buttonTypes = {
			text: `border-0 text-new.${this.color}-${this.color === 'gray' ? '800' : '400'}`,
			outline: `border border-new.${this.color}-${this.color === 'gray' ? '600' : '400'} text-new.${this.color}-${this.color === 'gray' ? '800' : '400'} rounded-8px`,
			softOutline: `border border-new.${this.color}-${this.color === 'gray' ? '300' : '50'} text-new.${this.color}-${this.color === 'gray' ? '800' : '400'} rounded-8px`,
			tab: `text-new.${this.color}-${this.color === 'gray' ? '800' : '400'} border-b-4px border-new.${this.color}-400`,
			link: `text-new.${this.color}-${this.color === 'gray' ? '800' : '400'} underline decoration-new.${this.color}-${this.color === 'gray' ? '600' : '400'}`,
			solid: `bg-new.${this.color}-400 text-white-100 border-none rounded-8px`,
			soft: `text-new.${this.color}-${this.color == 'gray' ? '600' : '400'} bg-new.${this.color}-${softColorLevels[this.color]} border-none rounded-8px`
		};

		return `${this.buttonSizes[this.size]} ${buttonTypes[this.fill]} ${this.fill === 'text' ? 'px-0' : ''} flex items-center justify-center disabled:opacity-50 whitespace-nowrap`;
	}
}
