import { Component, Input, OnInit } from '@angular/core';

@Component({
	selector: 'convoy-table-loader',
	templateUrl: './table-loader.component.html',
	styleUrls: ['./table-loader.component.scss']
})
export class TableLoaderComponent implements OnInit {
	@Input() tableHead!: string[];
	@Input() withDate = true;
	loaderIndex: number[] = [0, 1, 2, 3, 4];
	constructor() {}

	ngOnInit(): void {}
}
