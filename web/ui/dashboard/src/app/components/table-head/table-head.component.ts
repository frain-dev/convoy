import { Component, Input, OnInit } from '@angular/core';
import { CommonModule } from '@angular/common';

@Component({
	selector: 'convoy-table-head',
	standalone: true,
	imports: [CommonModule],
	templateUrl: './table-head.component.html',
	styleUrls: ['./table-head.component.scss']
})
export class TableHeadComponent implements OnInit {
	@Input('className') class!: string;

	constructor() {}

	ngOnInit(): void {}
}
