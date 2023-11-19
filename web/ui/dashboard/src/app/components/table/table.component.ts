import { Component, Directive, Input, OnInit } from '@angular/core';
import { CommonModule } from '@angular/common';

@Directive({
	selector: 'convoy-table, [convoy-table]',
	standalone: true,
	host: { class: 'w-full h-fit text-new.gray-600', id: 'table' }
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
		<div [class]="forDate ? 'pt-16px pl-16px pb-8px text-new.gray-400' : 'pt-12px pb-12px whitespace-nowrap text-new.gray-900'" class="flex flex-row items-center text-12 font-normal">
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
	host: { class: '' },
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
	host: { class: 'text-left whitespace-nowrap pt-10px pb-10px font-medium text-12 uppercase', scope: 'col' }
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
	host: { '[class]': 'getClassNames' }
})
export class TableRowComponent implements OnInit {
	@Input('forDate') forDate: boolean = false;
	@Input('active') active: boolean = false;

	constructor() {}

	ngOnInit(): void {}

	get getClassNames() {
		return `${this.forDate ? 'border-t border-new.primary-25 ' : 'hover:bg-new.primary-25 transition-all duration-300'} ${this.active ? 'bg-new.primary-25' : ''}`;
	}
}
