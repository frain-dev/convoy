import { Location } from '@angular/common';
import { Injectable } from '@angular/core';
import { ActivatedRoute } from '@angular/router';
import { BehaviorSubject } from 'rxjs';
import { FILTER_QUERY_PARAM } from 'src/app/models/event.model';
import { NOTIFICATION_STATUS } from 'src/app/models/global.model';
import { environment } from 'src/environments/environment';

@Injectable({
	providedIn: 'root'
})
export class GeneralService {
	alertStatus: BehaviorSubject<{ message: string; style: NOTIFICATION_STATUS; type?: string; show: boolean }> = new BehaviorSubject<{ message: string; style: NOTIFICATION_STATUS; type?: string; show: boolean }>({ message: 'testing', style: 'info', type: 'alert', show: false });

	queryParams: FILTER_QUERY_PARAM = {};
	constructor(private route: ActivatedRoute, private _location: Location) {}

	showNotification(details: { message: string; style: NOTIFICATION_STATUS; type?: string }) {
		this.alertStatus.next({ message: details.message, style: details.style, show: true, type: details.type ? details.type : 'alert' });
		setTimeout(() => {
			this.dismissNotification();
		}, 7000);
	}

	dismissNotification() {
		this.alertStatus.next({ message: '', style: 'info', show: false });
	}

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

		return { startDate, endDate };
	}

	getDate(date: Date) {
		const months = ['Jan', 'Feb', 'Mar', 'April', 'May', 'June', 'July', 'Aug', 'Sept', 'Oct', 'Nov', 'Dec'];
		const _date = new Date(date);
		const day = _date.getDate();
		const month = _date.getMonth();
		const year = _date.getFullYear();
		return `${day} ${months[month]}, ${year}`;
	}

	setContentDisplayed(content: { created_at: Date }[], sortOrder?: string) {
		const dateCreateds = content.map((item: { created_at: Date }) => this.getDate(item.created_at));
		const uniqueDateCreateds = [...new Set(dateCreateds)];
		let displayedItems: any = [];
		uniqueDateCreateds.forEach(itemDate => {
			const filteredItemDate = content.filter((item: { created_at: Date }) => this.getDate(item.created_at) === itemDate);
			const contents = { date: itemDate, content: filteredItemDate };
			displayedItems.push(contents);

			displayedItems = sortOrder === 'desc' ? displayedItems.sort((a: any, b: any) => Number(new Date(b.date)) - Number(new Date(a.date))) : displayedItems.sort((a: any, b: any) => Number(new Date(a.date)) - Number(new Date(b.date)));
		});

		return displayedItems;
	}

	addFilterToURL(params?: FILTER_QUERY_PARAM) {
		this.queryParams = { ...this.queryParams, ...this.route.snapshot.queryParams, ...params };

		if (!params?.next_page_cursor) delete this.queryParams.next_page_cursor;
		if (!params?.prev_page_cursor) delete this.queryParams.prev_page_cursor;

		const cleanedQuery: any = Object.fromEntries(Object.entries(this.queryParams).filter(([_, q]) => q !== '' && q !== undefined && q !== null));
		const queryParams = new URLSearchParams(cleanedQuery).toString();
		this._location.go(`${location.pathname}?${queryParams}`);

		return this.queryParams;
	}

	getCodeSnippetString(type: 'event_data' | 'res_body' | 'res_header' | 'req_header' | 'error' | 'log', data: any) {
		let displayMessage = '';
		switch (type) {
			case 'event_data':
				displayMessage = 'No event payload was sent';
				break;
			case 'res_body':
				displayMessage = 'No response body was sent';
				break;
			case 'res_header':
				displayMessage = 'No response header was sent';
				break;
			case 'req_header':
				displayMessage = 'No request header was sent';
				break;
			default:
				displayMessage = '';
				break;
		}

		if (data) return JSON.stringify(data, null, 4).replaceAll(/"([^"]+)":/g, '$1:');
		return displayMessage;
	}

	convertStringToJson(str: string) {
		try {
			const jsonObject = JSON.parse(str);
			return jsonObject;
		} catch {
			this.showNotification({ message: 'Data is not entered in correct JSON format', style: 'error' });
			return false;
		}
	}
}
