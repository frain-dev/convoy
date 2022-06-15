import { Component, EventEmitter, OnInit, Output } from '@angular/core';

@Component({
	selector: 'app-time-filter',
	templateUrl: './time-filter.component.html',
	styleUrls: ['./time-filter.component.scss']
})
export class TimeFilterComponent implements OnInit {
	showDropdown = false;
	filterStartHour: number = 0;
	filterEndHour: number = 23;
	filterStartMinute: number = 0;
	filterEndMinute: number = 59;
	timeFilterHours: number[] = [1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12];
	timeFilterMinutes: number[] = [0, 5, 10, 15, 20, 25, 30, 35, 40, 45, 50, 55, 59];
	isFilterUpdated = false;
	@Output('applyFilter') applyFilter: EventEmitter<any> = new EventEmitter();

	constructor() {}

	async ngOnInit() {}

	onApplyFilter() {
		this.isFilterUpdated = true;
		const startHour = this.filterStartHour < 10 ? `0${this.filterStartHour}` : `${this.filterStartHour}`;
		const startMinute = this.filterStartMinute < 10 ? `0${this.filterStartMinute}` : `${this.filterStartMinute}`;
		const endHour = this.filterEndHour < 10 ? `0${this.filterEndHour}` : `${this.filterEndHour}`;
		const endMinute = this.filterEndMinute < 10 ? `0${this.filterEndMinute}` : `${this.filterEndMinute}`;

		const startTime = `T${startHour}:${startMinute}:00`;
		const endTime = `T${endHour}:${endMinute}:59`;

		this.applyFilter.emit({
			startTime,
			endTime
		});
		this.showDropdown = false;
	}

	filterIsActive(): boolean {
		return !(this.filterStartHour === 0 && this.filterStartMinute === 0 && this.filterEndHour === 23 && this.filterEndMinute === 59);
	}

	clearFilter(event?: any) {
		event?.stopPropagation();

		this.filterStartHour = 0;
		this.filterEndHour = 23;
		this.filterStartMinute = 0;
		this.filterEndMinute = 59;
		this.onApplyFilter();
		this.showDropdown = false;
		this.isFilterUpdated = false;
	}
}
