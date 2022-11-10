import { Component, OnInit } from '@angular/core';
import { CommonModule } from '@angular/common';

@Component({
	selector: 'convoy-list-item, [convoy-list-item]',
	standalone: true,
	imports: [CommonModule],
	host: { class: 'flex items-center justify-between border-b border-grey-10 py-10px transition-all duration-300' },
	template: `
		<ng-content></ng-content>
	`
})
export class ListItemComponent implements OnInit {
	constructor() {}

	ngOnInit(): void {}
}
