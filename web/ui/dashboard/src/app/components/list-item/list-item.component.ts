import { Component, Input, OnInit } from '@angular/core';
import { CommonModule } from '@angular/common';

@Component({
	selector: 'convoy-list-item, [convoy-list-item]',
	standalone: true,
	host: { class: 'flex items-center justify-between py-10px transition-all duration-300 hover:bg-primary-500', '[class]': 'class' },
	imports: [CommonModule],
	template: `
		<ng-content></ng-content>
	`
})
export class ListItemComponent implements OnInit {
	@Input('hasBorder') hasBorder = true;
	@Input('active') active: 'true' | 'false' = 'false';

	constructor() {}

	ngOnInit(): void {}

	get class() {
		return `${this.hasBorder ? 'border-neutral-a3 border-b' : ''} ${this.active === 'true' ? 'bg-primary-500' : ''}`;
	}
}
