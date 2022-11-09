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

	show = false;

	constructor() {}

	ngOnInit(): void {}

}
