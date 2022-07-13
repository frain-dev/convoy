import { Component, Input, OnInit } from '@angular/core';
import { CommonModule } from '@angular/common';

@Component({
	selector: 'convoy-table-cell',
	standalone: true,
	imports: [CommonModule],
	templateUrl: './table-cell.component.html',
	styleUrls: ['./table-cell.component.scss']
})
export class TableCellComponent implements OnInit {
	@Input('className') class!: string;
	@Input('forDate') forDate: boolean = false;

	constructor() {}

	ngOnInit(): void {}
}
