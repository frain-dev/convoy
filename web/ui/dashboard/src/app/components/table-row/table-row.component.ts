import { Component, Input, OnInit } from '@angular/core';
import { CommonModule } from '@angular/common';

@Component({
	selector: 'convoy-table-row',
	standalone: true,
	imports: [CommonModule],
	templateUrl: './table-row.component.html',
	styleUrls: ['./table-row.component.scss']
})
export class TableRowComponent implements OnInit {
	@Input('className') class!: string;
	@Input('forDate') forDate: boolean = false;
	@Input('active') active: boolean = false;

	constructor() {}

	ngOnInit(): void {}

	get getClassNames() {
		return `${this.forDate ? 'border-t border-grey-10 ' : 'hover:bg-primary-500 transition-all duration-300'} ${this.active ? 'bg-primary-500' : ''} ${this.class}`;
	}
}
