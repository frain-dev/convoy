import { Component, Directive, Input, OnInit } from '@angular/core';
import { CommonModule } from '@angular/common';

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
