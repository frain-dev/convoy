import { Component, Input, OnInit } from '@angular/core';
import { CommonModule } from '@angular/common';

@Component({
	selector: 'convoy-table-head, [convoy-table-head]',
	standalone: true,
	imports: [CommonModule],
	host: { class: 'bg-primary-500' },
	template: `
		<tr>
			<ng-content></ng-content>
		</tr>
	`
})
export class TableHeadComponent implements OnInit {
	@Input('className') class!: string;

	constructor() {}

	ngOnInit(): void {}
}
