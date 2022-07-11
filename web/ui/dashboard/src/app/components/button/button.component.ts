import { Component, EventEmitter, Input, OnInit, Output } from '@angular/core';
import { CommonModule } from '@angular/common';

@Component({
	selector: 'convoy-button',
	standalone: true,
	imports: [CommonModule],
	templateUrl: './button.component.html',
	styleUrls: ['./button.component.scss']
})
export class ButtonComponent implements OnInit {
	@Input('disable') disable = false;
	@Input('buttonText') buttonText!: string;
	@Input('buttonType') buttonType!: 'button' | 'submit' | 'reset';
	@Input('className') class = '';
	@Input('size') size: 'tiny' | 'small' | 'medium' | 'full' = 'medium';
	@Input('type') type: 'default' | 'outline' | 'clear' | 'text' | 'link' | 'icon' = 'default';
	@Input('color') color: 'primary' | 'success' | 'warning' | 'danger' | 'grey' = 'primary';
	@Input('texture') texture: 'deep' | 'light' = 'deep';
	buttonSizes = { tiny: 'py-[1px] px-8px  text-12', small: `py-6px px-16px text-12`, medium: `py-12px px-40px`, full: `py-12px px-40px w-full` };
	buttonTypes: any = {};
	@Output('clickItem') click = new EventEmitter();

	constructor() {}

	ngOnInit(): void {}

	get classes(): string {
		const colorLevel = this.texture == 'deep' ? '100' : '500';
		this.buttonTypes = {
			default: `bg-${this.color}-${colorLevel} text-${this.texture == 'deep' ? 'white' : this.color}-100 border-none rounded-8px`,
			outline: `border rounded-[10px] border-${this.color}-${colorLevel} text-${this.color}-100 bg-transparent`,
			clear: `bg-transparent border-none text-${this.color}-100`,
			text: `bg-transparent border-none text-${this.color}-${colorLevel} ${this.size == 'small' ? 'text-12' : ''}`,
			link: `bg-transparent border-none text-${this.color}-${colorLevel} ${this.size == 'small' ? 'text-12' : ''} underline decoration-${this.color}-${colorLevel}`
		};
		return `${this.type !== 'text' && this.type !== 'icon' ? this.buttonSizes[this.size] : ''} ${this.buttonTypes[this.type]} ${this.class}`;
	}
}
