import { CommonModule } from '@angular/common';
import { Component, Input, OnInit } from '@angular/core';
import { CHARTDATA } from 'src/app/models/global.model';

@Component({
	selector: 'convoy-chart',
	standalone: true,
	imports: [CommonModule],
	templateUrl: './chart.component.html',
	styleUrls: ['./chart.component.scss']
})
export class ChartComponent implements OnInit {
    @Input('chartData') chartData!: CHARTDATA[];
	@Input('isLoading') isLoading: boolean = false;
	@Input('frequency') frequency: 'daily' | 'weekly' | 'monthly' | 'yearly' = 'daily';

	constructor() {}

	ngOnInit(): void {}

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
