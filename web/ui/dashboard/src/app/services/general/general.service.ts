import { Injectable } from '@angular/core';
import { BehaviorSubject } from 'rxjs';
import { environment } from 'src/environments/environment';

@Injectable({
	providedIn: 'root'
})
export class GeneralService {
	constructor() {}
	apiURL(): string {
		return `${environment.production ? location.origin : 'http://localhost:5005'}`;
	}

	getSelectedDate(dateOption: string) {
		const _date = new Date();
		let startDate, endDate, currentDayOfTheWeek;
		switch (dateOption) {
			case 'Last Year':
				startDate = new Date(_date.getFullYear() - 1, 0, 1);
				endDate = new Date(_date.getFullYear(), _date.getMonth(), _date.getDate());
				break;
			case 'Last Month':
				startDate = new Date(_date.getFullYear(), _date.getMonth() == 0 ? 11 : _date.getMonth() - 1, 1);
				endDate = new Date(_date.getFullYear(), _date.getMonth(), _date.getDate());
				break;
			case 'Last Week':
				currentDayOfTheWeek = _date.getDay();
				switch (currentDayOfTheWeek) {
					case 0:
						startDate = new Date(_date.getFullYear(), _date.getMonth(), _date.getDate() - 7);
						endDate = new Date(_date.getFullYear(), _date.getMonth(), _date.getDate());
						break;
					case 1:
						startDate = new Date(_date.getFullYear(), _date.getMonth(), _date.getDate() - 8);
						endDate = new Date(_date.getFullYear(), _date.getMonth(), _date.getDate());
						break;
					case 2:
						startDate = new Date(_date.getFullYear(), _date.getMonth(), _date.getDate() - 9);
						endDate = new Date(_date.getFullYear(), _date.getMonth(), _date.getDate());
						break;
					case 3:
						startDate = new Date(_date.getFullYear(), _date.getMonth(), _date.getDate() - 10);
						endDate = new Date(_date.getFullYear(), _date.getMonth(), _date.getDate());
						break;
					case 4:
						startDate = new Date(_date.getFullYear(), _date.getMonth(), _date.getDate() - 11);
						endDate = new Date(_date.getFullYear(), _date.getMonth(), _date.getDate());
						break;
					case 4:
						startDate = new Date(_date.getFullYear(), _date.getMonth(), _date.getDate() - 12);
						endDate = new Date(_date.getFullYear(), _date.getMonth(), _date.getDate());
						break;
					case 5:
						startDate = new Date(_date.getFullYear(), _date.getMonth(), _date.getDate() - 13);
						endDate = new Date(_date.getFullYear(), _date.getMonth(), _date.getDate());
						break;
					case 6:
						startDate = new Date(_date.getFullYear(), _date.getMonth(), _date.getDate() - 14);
						endDate = new Date(_date.getFullYear(), _date.getMonth(), _date.getDate());
						break;
					default:
						break;
				}
				break;
			case 'Yesterday':
				startDate = new Date(_date.getFullYear(), _date.getMonth(), _date.getDate() - 1);
				endDate = new Date(_date.getFullYear(), _date.getMonth(), _date.getDate());
				break;
			default:
				break;
		}

		return { startDate, endDate};
	}
}
