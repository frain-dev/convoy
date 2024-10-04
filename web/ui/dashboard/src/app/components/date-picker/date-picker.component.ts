import { Component, EventEmitter, Input, OnInit, Output } from '@angular/core';
import { CommonModule } from '@angular/common';
import { ButtonComponent } from '../button/button.component';
import { FormsModule } from '@angular/forms';
import { format, isAfter, isBefore, isFuture, isWithinInterval, parseISO } from 'date-fns';
import { DropdownContainerComponent } from '../dropdown-container/dropdown-container.component';
import { OverlayDirective } from '../overlay/overlay.directive';
import { InputDirective, InputFieldDirective, LabelComponent } from '../input/input.component';

interface CALENDAR_DAY {
	date: number;
	day: number;
	month: number;
	timestamp: number;
	dayString: string;
}

@Component({
	selector: 'convoy-date-picker',
	standalone: true,
	imports: [CommonModule, ButtonComponent, FormsModule, DropdownContainerComponent, OverlayDirective, InputFieldDirective, InputDirective, LabelComponent],
	templateUrl: './date-picker.component.html',
	styleUrls: ['./date-picker.component.scss']
})
export class DatePickerComponent implements OnInit {
	@Output() selectedDateRange = new EventEmitter<any>();
	@Output() selectedDate = new EventEmitter<any>();
	@Output() clearDates = new EventEmitter<any>();
	@Output() close = new EventEmitter<any>();
	@Input('show') show = false;
	@Input('formType') formType: 'filter' | 'form' = 'filter';
	@Input('position') position: 'right' | 'left' | 'center' | 'right-side' = 'left';
	@Input('dateRangeValue') dateRangeValue?: {
		startDate: string | Date;
		endDate: string | Date;
	};
	@Input('dateValue') dateValue?: string | Date;
	calendarDate!: CALENDAR_DAY[];
	oneDay = 60 * 60 * 24 * 1000;
	todayTimestamp = Date.now() - (Date.now() % this.oneDay) + new Date().getTimezoneOffset() * 1000 * 60;
	selectedStartDay? = this.todayTimestamp;
	selectedStartTime? = '00:00:00';
	selectedEndDay? = this.todayTimestamp;
	selectedEndTime? = '23:59:59';
	month!: number;
	year!: number;
	monthRight!: number;
	yearRight!: number;
	startDate = { day: '', month: '', year: '' };
	endDate = { day: '', month: '', year: '' };
	daysMap = ['Sunday', 'Monday', 'Tuesday', 'Wednesday', 'Thursday', 'Friday', 'Saturday'];
	monthMap = ['January', 'February', 'March', 'April', 'May', 'June', 'July', 'August', 'September', 'October', 'November', 'December'];
	selectedDates?: {
		startDate: string | Date;
		endDate: string | Date;
	};
	dateRangeValues = {
		startDate: `${format(new Date(this.selectedStartDay!), 'yyyy-MM-dd')}`,
		endDate: `${format(new Date(this.selectedEndDay!), 'yyyy-MM-dd')}`
	};

	showPicker = false;
	datesForLeftCalendar: CALENDAR_DAY[] = [];
	datesForRightCalendar: CALENDAR_DAY[] = [];

	filterStartHour: number = 0;
	filterEndHour: number = 23;
	filterStartMinute: number = 0;
	filterEndMinute: number = 59;
	timeFilterHours: number[] = [1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12];
	timeFilterMinutes: number[] = [0, 5, 10, 15, 20, 25, 30, 35, 40, 45, 50, 55, 59];

	constructor() {}

	ngOnInit(): void {
		if (this.dateRangeValue?.startDate && this.dateRangeValue?.endDate) {
			const startDate = new Date(this.dateRangeValue?.startDate);
			this.selectedStartDay = startDate.getTime();

			const endDate = new Date(this.dateRangeValue.endDate);
			this.selectedEndDay = new Date(endDate.getFullYear(), endDate.getMonth(), endDate.getDate()).getTime();

			this.selectedDates = { startDate: new Date(this.selectedStartDay!), endDate: new Date(this.selectedEndDay!) };
		}

		if (this.dateValue) this.selectedStartDay = new Date(this.dateValue).getTime();

		this.initDatePicker();

		if (this.show) this.showPicker = true;
	}

	initDatePicker() {
		let date = new Date();
		this.year = date.getFullYear();
		this.month = date.getMonth();

		this.yearRight = date.getFullYear();
		this.monthRight = date.getMonth() + 1;

		this.datesForLeftCalendar = this.getMonthDetails(this.year, this.month);
		this.datesForRightCalendar = this.getMonthDetails(this.yearRight, this.monthRight);
	}

	clearDate(event?: any) {
		event?.stopPropagation();
		delete this.selectedDates;
		this.selectedStartDay = this.todayTimestamp;
		this.selectedEndDay = this.todayTimestamp;
		this.initDatePicker();
		this.showPicker = false;
	}

	applyDate(applyDate: boolean = false) {
		if (!this.selectedStartDay && !this.selectedEndDay) return;

		this.dateRangeValues = { startDate: `${format(new Date(this.selectedStartDay!), 'yyyy-MM-dd')}`, endDate: `${this.selectedEndDay ? format(new Date(this.selectedEndDay!), 'yyyy-MM-dd') : ''}` };

		this.selectedDates = { startDate: `${format(new Date(this.selectedStartDay!), 'yyyy-MM-dd')}${this.selectedStartTime}`, endDate: `${this.selectedEndDay ? format(new Date(this.selectedEndDay!), 'yyyy-MM-dd') : ''}${this.selectedEndTime}` };

		if (applyDate) {
			this.showPicker = false;
			this.formType === 'filter' ? this.selectedDateRange.emit(this.selectedDates) : this.selectedDate.emit(this.selectedDates.startDate);
		}
	}

	onselectDay(timestamp: number) {
		if (this.formType === 'filter') {
			if (this.selectedStartDay && this.selectedEndDay) {
				this.selectedStartDay = timestamp;
				delete this.selectedEndDay;
			} else if (this.selectedStartDay && isBefore(new Date(timestamp), new Date(this.selectedStartDay))) {
				this.selectedEndDay = this.selectedStartDay;
				this.selectedStartDay = timestamp;
			} else if (this.selectedStartDay && isAfter(new Date(timestamp), new Date(this.selectedStartDay)) && !this.selectedEndDay) this.selectedEndDay = timestamp;
			else if (!this.selectedStartDay) this.selectedStartDay = timestamp;
			else if (this.selectedStartDay) this.selectedEndDay = timestamp;
		} else {
			this.selectedStartDay = timestamp;
			delete this.selectedEndDay;
		}

		this.applyDate();
	}

	setMonth(offset: number) {
		let year = this.year;
		let month = this.month + offset;

		let yearRight = this.yearRight;
		let monthRight = this.monthRight + offset;

		if (month <= -1) {
			month = 12 + month;
			year--;
			monthRight = 12 + monthRight;
			yearRight--;
		} else if (month >= 12) {
			month = month - 12;
			year++;
			monthRight = monthRight - 12;
			yearRight++;
		}

		this.year = year;
		this.month = month;
		this.yearRight = yearRight;
		this.monthRight = monthRight;

		this.datesForLeftCalendar = this.getMonthDetails(year, month);
		this.datesForRightCalendar = this.getMonthDetails(yearRight, monthRight);
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

	getMonthDetails(year: number, month: number): CALENDAR_DAY[] {
		let firstDay = new Date(year, month).getDay();
		let numberOfDays = this.getNumberOfDays(year, month);
		let monthArray = [];
		let rows = 6;
		let currentDay = null;
		let index = 0;
		let cols = 7;

		for (let row = 0; row < rows; row++) {
			for (let col = 0; col < cols; col++) {
				currentDay = this.getDayDetails({ index, numberOfDays, firstDay, year, month });
				monthArray.push(currentDay);
				index++;
			}
		}
		return monthArray;
	}

	isCurrentDay(timestamp: number): boolean {
		return timestamp === this.todayTimestamp;
	}

	isSelectedDay(timestamp: number) {
		return timestamp === this.selectedStartDay || timestamp === this.selectedEndDay;
	}

	isStartDay(timestamp: number) {
		return timestamp === this.selectedStartDay;
	}

	isEndDay(timestamp: number) {
		return timestamp === this.selectedEndDay;
	}

	isInFuture(timestamp: number) {
		return isFuture(new Date(timestamp));
	}

	isDayWithinStartAndEndDates(timestamp: number) {
		if (this.selectedStartDay && this.selectedEndDay) return isWithinInterval(new Date(timestamp), { start: new Date(this.selectedStartDay), end: new Date(this.selectedEndDay) });
		return false;
	}

	getDayClassNames(day: CALENDAR_DAY): string {
		const classNames = `w-full h-40px justify-center items-center transition-all duration-300 ease-in-out
        ${this.isCurrentDay(day.timestamp) && !this.isStartDay(day.timestamp) && !this.isEndDay(day.timestamp) ? '!bg-transparent !font-extrabold !text-primary-100' : ''}
        ${this.isDayWithinStartAndEndDates(day.timestamp) ? 'bg-primary-400 font-medium' : ''}
        ${(this.isInFuture(day.timestamp) && this.formType === 'filter') || day.month !== 0 ? 'opacity-30 pointer-events-none' : ''}
        ${day.month !== 0 ? '!opacity-0 pointer-events-none' : ''}
        ${this.isSelectedDay(day.timestamp) && day.month == 0 ? '!bg-primary-200 !text-white-100 font-medium' : ''}
        ${this.isStartDay(day.timestamp) ? 'rounded-bl-8px rounded-tl-8px' : ''}
        ${this.isEndDay(day.timestamp) ? 'rounded-br-8px rounded-tr-8px' : ''}`;
		return classNames;
	}

	getDayClassNamesRightCalendar(day: CALENDAR_DAY): string {
		const classNames = `w-full h-40px justify-center items-center transition-all duration-300 ease-in-out
        ${this.isCurrentDay(day.timestamp) && !this.isStartDay(day.timestamp) && !this.isEndDay(day.timestamp) ? '!bg-transparent !font-extrabold !text-primary-100' : ''}
        ${this.isDayWithinStartAndEndDates(day.timestamp) ? 'bg-primary-400 font-medium' : ''}
        ${day.month !== 0 ? '!opacity-0 pointer-events-none' : ''}
        ${this.isInFuture(day.timestamp) && this.formType === 'filter' ? 'opacity-30 pointer-events-none' : ''}
        ${this.isSelectedDay(day.timestamp) && day.month == 0 ? '!bg-primary-200 !text-white-100 font-medium' : ''}
        ${this.isStartDay(day.timestamp) ? 'rounded-bl-8px rounded-tl-8px' : ''}
        ${this.isEndDay(day.timestamp) ? 'rounded-br-8px rounded-tr-8px' : ''}`;
		return classNames;
	}

	formatDate(date: any) {
		const dateValue = new Date(date);
		return format(dateValue, 'yyyy-MM-dd');
	}

	// time filter functions
	onApplyFilter() {
		const startHour = this.filterStartHour < 10 ? `0${this.filterStartHour}` : `${this.filterStartHour}`;
		const startMinute = this.filterStartMinute < 10 ? `0${this.filterStartMinute}` : `${this.filterStartMinute}`;
		const endHour = this.filterEndHour < 10 ? `0${this.filterEndHour}` : `${this.filterEndHour}`;
		const endMinute = this.filterEndMinute < 10 ? `0${this.filterEndMinute}` : `${this.filterEndMinute}`;
		this.selectedStartTime = `T${startHour}:${startMinute}:00`;
		this.selectedEndTime = `T${endHour}:${endMinute}:59`;
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
	}

	validateTime(inputId: string) {
		const timeInputId = document.getElementById(inputId);
		const timeInputIdValue = document.getElementById(inputId) as HTMLInputElement;
		timeInputId?.addEventListener('keydown', e => {
			if (timeInputIdValue.value.length > 2) {
				if (!(e.key == 'Backspace' || e.key == 'Delete')) e.preventDefault();
			}
		});

		this.onApplyFilter();
	}
}
