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
	@Input('fill') fill: 'default' | 'outline' | 'clear' | 'text' | 'link' = 'default';
	@Input('size') size: 'xs' | 'sm' | 'md' | 'lg' = 'md';
	@Input('color') color: 'primary' | 'success' | 'warning' | 'danger' | 'grey' | 'transparent' = 'primary';
	@Input('texture') texture: 'deep' | 'light' = 'deep';
	@Input('index') tabIndex = 0;
	buttonSizes = { xs: 'py-4px px-8px text-12', sm: `py-6px px-16px text-12`, md: `py-10px px-36px text-14`, lg: `py-10px px-36px w-full text-14` };

	constructor() {}

	ngOnInit(): void {}

	get classes(): string {
		const colorLevel = this.texture == 'deep' ? '100' : this.color == 'grey' ? '20' : '500';
		const buttonTypes = {
			default: `bg-${this.color}-${colorLevel} text-${this.texture == 'deep' ? 'white' : this.color}-100 border-none rounded-8px`,
			outline: `border rounded-[10px] border-${this.color}-${colorLevel} text-${this.color}-100`,
			clear: `border-none text-${this.color}-100`,
			text: `border-0 text-${this.color}-${colorLevel} ${this.size == 'sm' || this.size == 'xs' ? 'text-12' : ''}`,
			link: `border-none text-${this.color}-${colorLevel} ${this.size == 'sm' || this.size == 'xs' ? 'text-12' : ''} underline decoration-${this.color}-${colorLevel}`
		};
		return `${this.fill !== 'text' ? this.buttonSizes[this.size] : ''} ${buttonTypes[this.fill]} flex items-center justify-center disabled:opacity-50`;
	}
}
