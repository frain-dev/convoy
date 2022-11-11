import { CommonModule } from '@angular/common';
import { Component, EventEmitter, Input, OnInit } from '@angular/core';
import { ButtonComponent } from '../button/button.component';

@Component({
	selector: 'convoy-dropdown',
	standalone: true,
	imports: [CommonModule, ButtonComponent],
	templateUrl: './dropdown.component.html',
	styleUrls: ['./dropdown.component.scss']
})
export class DropdownComponent implements OnInit {
	@Input('onSelectOption') onSelectOption = new EventEmitter();
	@Input('position') position: 'right' | 'left' | 'center' = 'right';
	@Input('size') size: 'sm' | 'md' | 'lg' | 'xl' | 'full' = 'md';
	@Input('active') active: boolean = false;
	@Input('className') class!: string;
	@Input('buttonText') buttonText!: string;
	@Input('buttonClass') buttonClass = '';
	@Input('buttonColor') buttonColor!: 'primary' | 'success' | 'warning' | 'danger' | 'grey';
	@Input('buttonSize') buttonSize: 'sm' | 'md' | 'lg' = 'md';
	@Input('buttonFill') buttonFill: 'default' | 'outline' | 'clear' | 'text' | 'link' = 'default';
	@Input('buttonTexture') buttonTexture: 'deep' | 'light' = 'deep';
	sizes = { sm: 'w-[140px]', md: 'w-[200px]', lg: 'w-[249px]', xl: 'w-[350px]', full: 'w-full' };
	show = false;

	constructor() {}

	ngOnInit(): void {}

	get classes(): string {
		const positions = {
			right: 'right-[5%]',
			left: 'left-[5%]',
			center: 'left-0'
		};
		return `${this.sizes[this.size]} ${positions[this.position]} ${this.show ? 'opacity-100 h-fit overflow-y-auto pointer-events-auto' : 'opacity-0 h-0 overflow-hidden pointer-events-none'} ${this.class}`;
	}

	get buttonClasses(): string {
		return `${this.active ? 'text-primary-100 !bg-primary-500' : ''} empty:hidden ${this.buttonClass}`;
	}
}
