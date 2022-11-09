import { Component, Input, OnInit } from '@angular/core';
import { CommonModule } from '@angular/common';

@Component({
	selector: 'convoy-table-cell, [convoy-table-cell]',
	standalone: true,
	imports: [CommonModule],
	host: { class: 'p-0 ' },
	template: `
		<div [class]="forDate ? 'pt-16px pl-16px pb-8px !text-12 text-grey-40' : 'pt-12px pb-12px whitespace-nowrap text-14'" class="flex flex-row items-center">
			<ng-content></ng-content>
		</div>
	`
})
export class TableCellComponent implements OnInit {
	@Input('forDate') forDate: boolean = false;

	constructor() {}

	ngOnInit(): void {}
}
