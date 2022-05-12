import { Component, EventEmitter, OnInit, Output } from '@angular/core';

@Component({
	selector: 'time-filter',
	template: `
		<div class="dropdown margin-left__24px">
			<button class="button button__filter button--has-icon icon-left" (click)="showDropdown = !showDropdown">
				<img src="/assets/img/clock.svg" alt="time filter icon" />
				<span class="font__weight-500">{{ filterStartHour === 0 ? 12 : filterStartHour > 12 ? filterStartHour - 12 : filterStartHour }}</span>
				<span class="font__weight-500">:</span>
				<span class="font__weight-500">{{ filterStartMinute < 10 ? '0' + filterStartMinute : filterStartMinute }}</span>
				<span class="font__weight-500">{{ filterStartHour >= 12 ? ' pm' : ' am' }}</span>
				<span class="margin-left__6px margin-right__6px font__weight-500">-</span>
				<span class="font__weight-500">{{ filterEndHour > 12 ? filterEndHour - 12 : filterEndHour }}</span>
				<span class="font__weight-500">:</span>
				<span class="font__weight-500">{{ filterEndMinute < 10 ? '0' + filterEndMinute : filterEndMinute }}</span>
				<span class="font__weight-500">{{ filterEndHour >= 12 ? ' pm' : ' am' }}</span>
				<img class="margin-left__16px margin-right__0px" src="/assets/img/angle-arrow-down.svg" alt="arrow down icon" />
			</button>

			<div class="dropdown__menu time-filter with-padding" [ngClass]="{ show: showDropdown }">
				<div class="flex">
					<div class="border__right margin-right__16px padding-right__10px">
						<h5 class="padding-bottom__10px border__bottom">Start Date Time</h5>

						<div class="flex flex__align-items-center">
							<ul class="can-scroll">
								<li
									*ngFor="let hour of timeFilterHours"
									(click)="filterStartHour = filterStartHour >= 12 ? hour + 12 : hour"
									[ngClass]="{ active: (filterStartHour > 12 ? filterStartHour - 12 : filterStartHour) === hour || (filterStartHour === 0 && hour === 12) }"
								>
									{{ hour < 10 ? '0' + hour : hour }}
								</li>
							</ul>

							<ul class="can-scroll">
								<li *ngFor="let minute of timeFilterMinutes; let i = index" (click)="filterStartMinute = minute" [ngClass]="{ active: filterStartMinute === minute }">
									{{ minute < 10 ? '0' + minute : minute }}
								</li>
							</ul>

							<ul>
								<li (click)="filterStartHour = filterStartHour - 12 < 0 ? filterStartHour : filterStartHour - 12" [ngClass]="{ active: filterStartHour < 12 }">am</li>
								<li (click)="filterStartHour = filterStartHour + 12 > 24 ? filterStartHour : filterStartHour + 12" [ngClass]="{ active: filterStartHour >= 12 }">pm</li>
							</ul>
						</div>
					</div>

					<div>
						<h5 class="padding-bottom__10px border__bottom">End Date Time</h5>

						<div class="flex flex__align-items-center">
							<ul class="can-scroll">
								<li
									*ngFor="let hour of timeFilterHours"
									(click)="filterEndHour = filterEndHour >= 12 ? hour + 12 : hour"
									[ngClass]="{ active: (filterEndHour > 12 ? filterEndHour - 12 : filterEndHour) === hour }"
								>
									{{ hour < 10 ? '0' + hour : hour }}
								</li>
							</ul>

							<ul class="can-scroll">
								<li *ngFor="let minute of timeFilterMinutes; let i = index" (click)="filterEndMinute = minute" [ngClass]="{ active: filterEndMinute === minute }">
									{{ minute < 10 ? '0' + minute : minute }}
								</li>
							</ul>

							<ul>
								<li (click)="filterEndHour = filterEndHour - 12 < 0 ? filterEndHour : filterEndHour - 12" [ngClass]="{ active: filterEndHour < 12 }">am</li>
								<li (click)="filterEndHour = filterEndHour + 12 > 24 ? filterEndHour : filterEndHour + 12" [ngClass]="{ active: filterEndHour >= 12 }">pm</li>
							</ul>
						</div>
					</div>
				</div>

				<div class="margin-top__16px flex flex__align-items-center">
					<button class="button button__primary button__small" (click)="onApplyFilter()">Apply</button>
					<button class="button button__clear button__small margin-left__10px" (click)="clearFilter()">Clear</button>
				</div>
			</div>
		</div>
		<div class="overlay" *ngIf="showDropdown" (click)="showDropdown = false"></div>
	`,
	styles: [
		`
			.time-filter {
				width: fit-content;
			}

			.can-scroll {
				height: 200px;
				overflow-y: scroll;
			}

			.can-scroll::-webkit-scrollbar {
				width: 2px;
			}

			.can-scroll::-webkit-scrollbar-track {
				background: #f3f3f3;
			}
		`
	]
})
export class TimeFilterComponent implements OnInit {
	showDropdown = false;
	filterStartHour: number = 0;
	filterEndHour: number = 23;
	filterStartMinute: number = 0;
	filterEndMinute: number = 59;
	timeFilterHours: number[] = [1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12];
	timeFilterMinutes: number[] = [0, 5, 10, 15, 20, 25, 30, 35, 40, 45, 50, 55, 59];
	@Output('applyFilter') applyFilter: EventEmitter<any> = new EventEmitter();

	constructor() {}
	async ngOnInit() {}

	onApplyFilter() {
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

	clearFilter() {
		this.filterStartHour = 0;
		this.filterEndHour = 23;
		this.filterStartMinute = 0;
		this.filterEndMinute = 59;
		this.onApplyFilter();
		this.showDropdown = false;
	}
}
