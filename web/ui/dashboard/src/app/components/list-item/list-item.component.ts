import { Component, Input, OnInit } from '@angular/core';
import { CommonModule } from '@angular/common';

@Component({
	selector: 'convoy-list-item, [convoy-list-item]',
	standalone: true,
	host: { class: 'flex items-center justify-between py-10px transition-all duration-300', '[class]': "hasBorder?'border-b border-grey-10':''" },
	imports: [CommonModule],
	template: `
		<ng-content></ng-content>
	`
})
export class ListItemComponent implements OnInit {
	@Input('hasBorder') hasBorder = true;
	constructor() {}

	ngOnInit(): void {}
}
