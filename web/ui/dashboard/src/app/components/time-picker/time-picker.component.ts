import { Component, EventEmitter, OnInit, Output } from '@angular/core';
import { CommonModule } from '@angular/common';
import { ButtonComponent } from '../button/button.component';
import { FormsModule } from '@angular/forms';
import { DropdownContainerComponent } from '../dropdown-container/dropdown-container.component';
import { ScreenDirective } from '../screen/screen.directive';

@Component({
	selector: 'convoy-time-picker',
	standalone: true,
	imports: [CommonModule, ButtonComponent, FormsModule, DropdownContainerComponent, ScreenDirective],
	templateUrl: './time-picker.component.html',
	styleUrls: ['./time-picker.component.scss']
})
export class TimePickerComponent implements OnInit {
	filterStartHour: number = 0;
	filterEndHour: number = 23;
	filterStartMinute: number = 0;
	filterEndMinute: number = 59;
	timeFilterHours: number[] = [1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12];
	timeFilterMinutes: number[] = [0, 5, 10, 15, 20, 25, 30, 35, 40, 45, 50, 55, 59];
	isFilterUpdated = false;
	showPicker = false;
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
		this.showPicker = false;
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
		this.isFilterUpdated = false;
	}

	validateTime(inputId: string) {
		const timeInputId = document.getElementById(inputId);
		const timeInputIdValue = document.getElementById(inputId) as HTMLInputElement;
		timeInputId?.addEventListener('keydown', e => {
			if (timeInputIdValue.value.length > 2) {
				if (!(e.key == 'Backspace' || e.key == 'Delete')) e.preventDefault();
			}
		});
	}
}
