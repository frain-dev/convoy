import { CommonModule } from '@angular/common';
import { Component, EventEmitter, Input, OnInit } from '@angular/core';
import { ButtonComponent } from '../button/button.component';
import { DropdownContainerComponent } from '../dropdown-container/dropdown-container.component';

@Component({
	selector: 'convoy-dropdown, [convoy-dropdown]',
	standalone: true,
	imports: [CommonModule, ButtonComponent, DropdownContainerComponent],
	templateUrl: './dropdown.component.html',
	styleUrls: ['./dropdown.component.scss']
})
export class DropdownComponent implements OnInit {
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
	show = false;

	constructor() {}

	ngOnInit(): void {}

	get buttonClasses(): string {
		return `${this.active ? 'text-primary-100 !bg-primary-500' : ''} empty:hidden ${this.buttonClass}`;
	}
}
