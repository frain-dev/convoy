import { CommonModule } from '@angular/common';
import { Component, Input, OnInit } from '@angular/core';
import { CHARTDATA } from 'src/app/models/global.model';
import { ButtonComponent } from '../button/button.component';

@Component({
	selector: 'convoy-chart',
	standalone: true,
	imports: [CommonModule, ButtonComponent],
	templateUrl: './chart.component.html',
	styleUrls: ['./chart.component.scss']
})
export class ChartComponent implements OnInit {
    @Input('chartData') chartData!: CHARTDATA[];
	@Input('isLoading') isLoading: boolean = false;
	@Input('frequency') frequency: 'daily' | 'weekly' | 'monthly' | 'yearly' = 'daily';
    paginatedData!: CHARTDATA[];
	pageSize = 30;
	pageNumber = 1;
	pages = 1;

	constructor() {}

	ngOnInit() {
		this.paginateChartData();
	}

	generateRandomHeight(maxHeight: number) {
		const randomNum = Math.floor(Math.random() * maxHeight);
		return randomNum;
	}

	paginateChartData() {
		this.pages = Math.ceil(this.chartData.length / this.pageSize);
		this.paginate();
	}

	prevPage() {
		if (this.pageNumber === 1) return;
		this.pageNumber--;
		this.paginate();
	}
	nextPage() {
		if (this.pageNumber === this.pages) return;
		this.pageNumber++;
		this.paginate();
	}

	paginate() {
		this.paginatedData = this.chartData.slice((this.pageNumber - 1) * this.pageSize, this.pageNumber * this.pageSize);
	}

	counter(i: number) {
		return new Array(i);
	}
	// generateHeight() {
	// 	const maxHeight = 100;
	// 	const loaders = document.querySelectorAll('div.loader');
	// 	var randomNum;
	// 	if (loaders !== null) {
	// 		loaders.forEach(loader => {
	// 			do {
	// 				randomNum = this.generateRandomHeight(maxHeight);
	// 			} while (randomNum < maxHeight);
	// 			loader.style.height = randomNum + 'px';
	// 		});
	// 	}
	// }
}
