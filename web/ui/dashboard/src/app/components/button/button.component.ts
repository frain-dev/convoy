import { ChangeDetectionStrategy, Component, EventEmitter, Input, OnInit, Output } from '@angular/core';
import { CommonModule } from '@angular/common';

@Component({
	selector: 'convoy-button',
	standalone: true,
	imports: [CommonModule],
	templateUrl: './button.component.html',
	styleUrls: ['./button.component.scss'],
	changeDetection: ChangeDetectionStrategy.OnPush
})
export class ButtonComponent implements OnInit {
	@Input('disable') disable = false;
	@Input('buttonText') buttonText!: string;
	@Input('buttonType') buttonType!: 'button' | 'submit' | 'reset';
	@Input('className') class = '';
	@Input('size') size: 'xs' | 'sm' | 'md' | 'lg' = 'md';
	@Input('type') type: 'default' | 'outline' | 'clear' | 'text' | 'link' | 'icon' | 'unstyled' = 'default';
	@Input('color') color: 'primary' | 'success' | 'warning' | 'danger' | 'grey' = 'primary';
	@Input('texture') texture: 'deep' | 'light' = 'deep';
	@Input('index') tabIndex = 0;
	buttonSizes = { xs: 'py-[1px] px-8px  text-12', sm: `py-6px px-16px text-12`, md: `py-10px px-36px text-14`, lg: `py-10px px-36px w-full text-14` };
	buttonTypes: any = {};
	@Output('clickItem') click = new EventEmitter();

	constructor() {}

	ngOnInit(): void {}

	get classes(): string {
		const colorLevel = this.texture == 'deep' ? '100' : '500';
		this.buttonTypes = {
			default: `bg-${this.color}-${colorLevel} text-${this.texture == 'deep' ? 'white' : this.color}-100 border-none rounded-8px`,
			outline: `border rounded-[10px] border-${this.color}-${colorLevel} text-${this.color}-100`,
			clear: `border-none text-${this.color}-100`,
			text: `border-0 text-${this.color}-${colorLevel} ${this.size == 'sm' || this.size == 'xs' ? 'text-12' : ''}`,
			link: `border-none text-${this.color}-${colorLevel} ${this.size == 'sm' || this.size == 'xs' ? 'text-12' : ''} underline decoration-${this.color}-${colorLevel}`,
			unstyled: ''
		};
		return `${this.type !== 'text' && this.type !== 'icon' && this.type !== 'unstyled' ? this.buttonSizes[this.size] : ''} ${this.buttonTypes[this.type]} ${this.class}`;
	}
}
