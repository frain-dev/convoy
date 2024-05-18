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
		warning: 'bg-warning-a3 text-warning-9',
		error: 'bg-error-a3 text-error-9',
		info: 'bg-new.primary-25 text-primary-400',
		success: 'bg-success-a3 text-success-9'
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
