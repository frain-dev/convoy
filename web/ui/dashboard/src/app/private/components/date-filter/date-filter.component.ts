import { Component, EventEmitter, Input, OnInit, Output, ViewChild } from '@angular/core';
import { isAfter, isBefore, isFuture, isWithinInterval } from 'date-fns';
import { DropdownComponent } from 'src/stories/dropdown/dropdown.component';

interface CALENDAR_DAY {
	date: number;
	day: number;
	month: number;
	timestamp: number;
	dayString: string;
}

@Component({
	selector: 'date-filter',
	templateUrl: './date-filter.component.html',
	styleUrls: ['./date-filter.component.scss']
})
export class DateFilterComponent implements OnInit {
	@ViewChild('dropdown') dropdown!: DropdownComponent;
	@Output() selectedDateRange = new EventEmitter<any>();
	@Output() clearDates = new EventEmitter<any>();
	@Input('dateRangeValue') dateRangeValue?: {
		startDate: string | Date;
		endDate: string | Date;
	};
	calendarDate!: CALENDAR_DAY[];
	oneDay = 60 * 60 * 24 * 1000;
	todayTimestamp = Date.now() - (Date.now() % this.oneDay) + new Date().getTimezoneOffset() * 1000 * 60;
	selectedStartDay? = this.todayTimestamp;
	selectedEndDay? = this.todayTimestamp;
	month!: number;
	year!: number;
	startDate = { day: '', month: '', year: '' };
	endDate = { day: '', month: '', year: '' };
	daysMap = ['Sunday', 'Monday', 'Tuesday', 'Wednesday', 'Thursday', 'Friday', 'Saturday'];
	monthMap = ['January', 'February', 'March', 'April', 'May', 'June', 'July', 'August', 'September', 'October', 'November', 'December'];
	selectedDates?: {
		startDate: string | Date;
		endDate: string | Date;
	};

	constructor() {}

	ngOnInit(): void {
		if (this.dateRangeValue) {
			this.selectedStartDay = new Date(this.dateRangeValue.startDate).getTime();
			const endDate = new Date(this.dateRangeValue.endDate);
			this.selectedEndDay = new Date(endDate.getFullYear(), endDate.getMonth(), endDate.getDate()).getTime();

			if (this.dateRangeValue.startDate && this.dateRangeValue.endDate) this.selectedDates = { startDate: new Date(this.selectedStartDay!), endDate: new Date(this.selectedEndDay!) };
		}

		this.initDatePicker();
	}

	initDatePicker() {
		let date = new Date();
		this.year = date.getFullYear();
		this.month = date.getMonth();
		this.getMonthDetails(this.year, this.month);

		this.setInputStartDate();
		this.setInputEndDate();
	}

	clearDate(event?: any) {
		event?.stopPropagation();
		delete this.selectedDates;
		this.selectedStartDay = this.todayTimestamp;
		this.selectedEndDay = this.todayTimestamp;
		this.initDatePicker();
		this.dropdown.show = false;
	}

	applyDate() {
		this.selectedDates = { startDate: new Date(this.selectedStartDay!), endDate: new Date(this.selectedEndDay!) };
		this.selectedDateRange.emit(this.selectedDates);
		this.dropdown.show = false;
	}

	get getCalculatedClass() {
		return `${this.selectedDates?.startDate && this.selectedDates?.endDate ? 'text-primary-100 !bg-primary-500 ' : ''}`;
	}

	setInputStartDate() {
		if (this.selectedStartDay) {
			const date = new Date(this.selectedStartDay);
			this.startDate = { day: date.getDate() < 9 ? '0' + date.getDate() : String(date.getDate()), month: date.getMonth() + 1 < 9 ? '0' + (date.getMonth() + 1) : String(date.getMonth() + 1), year: String(date.getFullYear()) };
		}
	}

	setInputEndDate() {
		if (this.selectedEndDay) {
			const date = new Date(this.selectedEndDay);
			this.endDate = { day: date.getDate() < 9 ? '0' + date.getDate() : String(date.getDate()), month: date.getMonth() + 1 < 9 ? '0' + (date.getMonth() + 1) : String(date.getMonth() + 1), year: String(date.getFullYear()) };
		}
	}

	onInputStartDate() {
		const timestamp = new Date(parseInt(this.startDate.year), parseInt(this.startDate.month) - 1, parseInt(this.startDate.day)).getTime();
		const date = new Date(timestamp);
		if (this.selectedEndDay) if (isAfter(date, new Date(this.selectedEndDay))) return;
		if (!this.isInFuture(timestamp)) this.selectedStartDay = timestamp;
		else return;

		this.year = date.getFullYear();
		this.month = date.getMonth();
		this.getMonthDetails(this.year, this.month);
	}

	onInputEndDate() {
		const timestamp = new Date(parseInt(this.endDate.year), parseInt(this.endDate.month) - 1, parseInt(this.endDate.day)).getTime();
		const date = new Date(timestamp);
		if (this.selectedStartDay) if (isBefore(date, new Date(this.selectedStartDay))) return;
		if (!this.isInFuture(timestamp)) this.selectedEndDay = timestamp;
		else return;

		this.year = date.getFullYear();
		this.month = date.getMonth();
		this.getMonthDetails(this.year, this.month);
	}

	onselectDay(timestamp: number) {
		if (this.selectedStartDay && this.selectedEndDay) {
			this.selectedStartDay = timestamp;
			delete this.selectedEndDay;
		} else if (this.selectedStartDay && isBefore(new Date(timestamp), new Date(this.selectedStartDay))) this.selectedStartDay = timestamp;
		else if (this.selectedStartDay && isAfter(new Date(timestamp), new Date(this.selectedStartDay)) && !this.selectedEndDay) this.selectedEndDay = timestamp;
		else if (!this.selectedStartDay) this.selectedStartDay = timestamp;

		this.setInputStartDate();
		this.setInputEndDate();
	}

	setYear(offset: number) {
		let year = this.year + offset;
		let month = this.month;

		this.year = year;
		this.month = month;
		this.getMonthDetails(year, month);
	}

	setMonth(offset: number) {
		let year = this.year;
		let month = this.month + offset;

		if (month === -1) {
			month = 11;
			year--;
		} else if (month === 12) {
			month = 0;
			year++;
		}

		this.year = year;
		this.month = month;
		this.getMonthDetails(year, month);
	}

	getDayDetails(args: { index: any; numberOfDays: any; firstDay: any; year: any; month: any }) {
		let date = args.index - args.firstDay;
		let day = args.index % 7;
		let prevMonth = args.month - 1;
		let prevYear = args.year;
		if (prevMonth < 0) {
			prevMonth = 11;
			prevYear--;
		}
		let prevMonthNumberOfDays = this.getNumberOfDays(prevYear, prevMonth);
		let _date = (date < 0 ? prevMonthNumberOfDays + date : date % args.numberOfDays) + 1;
		let month = date < 0 ? -1 : date >= args.numberOfDays ? 1 : 0;
		let timestamp = new Date(args.year, args.month, _date).getTime();
		return {
			date: _date,
			day,
			month,
			timestamp,
			dayString: this.daysMap[day]
		};
	}

	getNumberOfDays(year: number, month: number) {
		return 40 - new Date(year, month, 40).getDate();
	}

	getMonthDetails(year: number, month: number) {
		let firstDay = new Date(year, month).getDay();
		let numberOfDays = this.getNumberOfDays(year, month);
		let monthArray = [];
		let rows = 6;
		let currentDay = null;
		let index = 0;
		let cols = 7;

		for (let row = 0; row < rows; row++) {
			for (let col = 0; col < cols; col++) {
				currentDay = this.getDayDetails({
					index,
					numberOfDays,
					firstDay,
					year,
					month
				});
				monthArray.push(currentDay);
				index++;
			}
		}
		this.calendarDate = monthArray;
	}

	isCurrentDay(timestamp: number): boolean {
		return timestamp === this.todayTimestamp;
	}

	isSelectedDay(timestamp: number) {
		return timestamp === this.selectedStartDay || timestamp === this.selectedEndDay;
	}

	isInFuture(timestamp: number) {
		return isFuture(new Date(timestamp));
	}

	isDayWithinStartAndEndDates(timestamp: number) {
		if (this.selectedStartDay && this.selectedEndDay) return isWithinInterval(new Date(timestamp), { start: new Date(this.selectedStartDay), end: new Date(this.selectedEndDay) });
		return false;
	}

	getDayClassNames(day: CALENDAR_DAY): string {
		const classNames = `w-40px h-40px justify-center items-center transition-all duration-300 ease-in-out ${this.isCurrentDay(day.timestamp) ? '!border border-primary-100' : ''} ${day.month !== 0 ? 'hidden' : ''} ${
			this.isDayWithinStartAndEndDates(day.timestamp) ? 'bg-primary-400 font-medium' : ''
		} ${this.isInFuture(day.timestamp) ? 'opacity-30 pointer-events-none' : ''} ${this.isSelectedDay(day.timestamp) && day.month == 0 ? '!bg-primary-200 !text-white-100  font-medium' : ''}`;
		return classNames;
	}
}
