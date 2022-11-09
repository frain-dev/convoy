import { Component, Directive, Input, OnInit } from '@angular/core';
import { CommonModule } from '@angular/common';

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
