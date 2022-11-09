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
