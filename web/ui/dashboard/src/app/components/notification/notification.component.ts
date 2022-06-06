import { Component, OnInit } from '@angular/core';
import { GeneralService } from 'src/app/services/general/general.service';

@Component({
	selector: 'app-notification',
	templateUrl: './notification.component.html',
	styleUrls: ['./notification.component.scss']
})
export class NotificationComponent implements OnInit {
	notification!: { message: string; style: string; show: boolean };
	constructor(private generalService: GeneralService) {}

	async ngOnInit() {
		await this.triggerNotification();
	}

	triggerNotification() {
		this.generalService.alertStatus.subscribe(res => {
			this.notification = res;
		});
	}
	dismissNotification() {
		this.generalService.dismissNotification();
	}
}
