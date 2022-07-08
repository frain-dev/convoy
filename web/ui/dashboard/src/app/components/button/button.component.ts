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
	@Input('class') class = '';
	@Input('size') size: 'small' | 'medium' | 'full' = 'medium';
	@Input('type') type: 'default' | 'outline' | 'clear' | 'text' | 'link' = 'default';
	@Input('color') color: 'primary' | 'success' | 'warning' | 'danger' = 'primary';
	buttonSizes = { small: `py-6px px-16px`, medium: `py-12px px-40px`, full: `py-12px px-40px w-full` };
	buttonTypes = {
		default: `bg-${this.color}-100 text-white-100 border-none rounded-8px`,
		outline: `border rounded-8px border-${this.color}-200 text-${this.color}-100 bg-transparent`,
		clear: `bg-transparent border-none`,
		text: `bg-transparent border-none text-${this.color}-100`,
		link: `bg-transparent border-none text-${this.color}-100 underline decoration-primary-100`
	};
	@Output('click') click = new EventEmitter();

	constructor() {}

	ngOnInit(): void {}

	get classes(): string {
		return `${this.buttonSizes[this.size]} ${this.buttonTypes[this.type]} ${this.class}`;
	}
}
