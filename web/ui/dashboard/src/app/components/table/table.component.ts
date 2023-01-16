import { Component, Directive, Input, OnInit } from '@angular/core';
import { CommonModule } from '@angular/common';

@Directive({
	selector: 'convoy-table, [convoy-table]',
	standalone: true,
	host: { class: 'w-full h-fit', id: 'table' }
})
export class TableComponent implements OnInit {
	constructor() {}

	ngOnInit(): void {}
}

/* ============== Table Cell ============== */
@Component({
	selector: 'convoy-table-cell, [convoy-table-cell]',
	standalone: true,
	imports: [CommonModule],
	host: { class: 'p-0 ' },
	template: `
		<div [class]="forDate ? 'pt-16px pl-16px pb-8px !text-12 text-grey-40' : 'pt-12px pb-12px whitespace-nowrap text-14'" class="flex flex-row items-center font-light text-grey-100">
			<ng-content></ng-content>
		</div>
	`
})
export class TableCellComponent implements OnInit {
	@Input('forDate') forDate: boolean = false;

	constructor() {}

	ngOnInit(): void {}
}

/* ============== Table Head ============== */
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

/* ============== Table Head Cell ============== */
@Directive({
	selector: 'convoy-table-head-cell, [convoy-table-head-cell]',
	standalone: true,
	host: { class: 'text-left whitespace-nowrap pt-10px pb-10px font-medium text-12 capitalize text-grey-100', scope: 'col' }
})
export class TableHeadCellComponent implements OnInit {
	@Input('className') class!: string;

	constructor() {}

	ngOnInit(): void {}
}

/* ============== Table Row ============== */
@Directive({
	selector: 'convoy-table-row, [convoy-table-row]',
	standalone: true,
	host: { '[class]': 'getClassNames', class: 'cursor-pointer' }
})
export class TableRowComponent implements OnInit {
	@Input('forDate') forDate: boolean = false;
	@Input('active') active: boolean = false;

	constructor() {}

	ngOnInit(): void {}

	get getClassNames() {
		return `${this.forDate ? 'border-t border-grey-10 ' : 'hover:bg-primary-500 transition-all duration-300'} ${this.active ? 'bg-primary-500' : ''}`;
	}
}
