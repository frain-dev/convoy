import { CommonModule } from '@angular/common';
import { Component, OnInit } from '@angular/core';
import { NOTIFICATION_STATUS } from 'src/app/models/global.model';
import { GeneralService } from 'src/app/services/general/general.service';
import { ButtonComponent } from '../button/button.component';

@Component({
	selector: 'convoy-notification',
	standalone: true,
	imports: [CommonModule, ButtonComponent],
	templateUrl: './notification.component.html',
	styleUrls: ['./notification.component.scss']
})
export class NotificationComponent implements OnInit {
	notification!: { message: string; style: NOTIFICATION_STATUS; type?: string; show: boolean };
	statusTypes = {
		warning: 'bg-warning-500 text-warning-100',
		error: 'bg-danger-500 text-danger-100',
		info: 'bg-primary-500 text-primary-100',
		success: 'bg-success-500 text-success-100'
	};
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
