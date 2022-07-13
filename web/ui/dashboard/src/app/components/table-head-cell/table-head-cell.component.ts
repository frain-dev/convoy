import { Component, Input, OnInit } from '@angular/core';
import { CommonModule } from '@angular/common';

@Component({
	selector: 'convoy-table-head-cell',
	standalone: true,
	imports: [CommonModule],
	templateUrl: './table-head-cell.component.html',
	styleUrls: ['./table-head-cell.component.scss']
})
export class TableHeadCellComponent implements OnInit {
	@Input('className') class!: string;

	constructor() {}

	ngOnInit(): void {}
}
