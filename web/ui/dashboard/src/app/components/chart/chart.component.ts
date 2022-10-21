import { CommonModule } from '@angular/common';
import { Component, Input, OnInit, SimpleChanges } from '@angular/core';
import { CHARTDATA } from 'src/app/models/global.model';
import { ButtonComponent } from '../button/button.component';
interface PAGE_DATA extends CHARTDATA {
	size: string;
}
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
	paginatedData: PAGE_DATA[] = [];
	pageSize = 31;
	pageNumber = 1;
	pages = 1;
	loaderSizes!: number[];

	constructor() {}

	ngOnInit() {
		this.generateLoaderHeight();
	}

	ngOnChanges(changes: SimpleChanges) {
		this.isLoading = changes?.isLoading?.currentValue;
		this.chartData = changes?.chartData?.currentValue;
		if (changes?.isLoading?.previousValue !== changes?.isLoading?.currentValue) this.pageNumber = 1;
		if (this.chartData) this.paginateChartData();
	}

	paginateChartData() {
		this.pages = Math.ceil(this.chartData?.length / this.pageSize);
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
		const chartData = this.chartData.slice((this.pageNumber - 1) * this.pageSize, this.pageNumber * this.pageSize);
		const dataSet: number[] = chartData.map(data => data.data);
		const maxData = Math.max(...dataSet);

		this.paginatedData = chartData.map(data => {
			return { ...data, size: `${Math.round((100 / maxData) * data.data) || 4}px` };
		});
	}

	generateLoaderHeight() {
		this.loaderSizes = Array.from({ length: 30 }, () => Math.floor(Math.random() * 100));
	}
}
