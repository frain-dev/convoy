import { ChangeDetectionStrategy, Component, OnInit } from '@angular/core';
import { ConvoyDashboardService } from '../convoy-dashboard.service';

@Component({
	selector: 'convoy-notification',
	changeDetection: ChangeDetectionStrategy.Default,
	template: `
		<div *ngIf="notification.show" class="toast toast--{{ notification.style }} z-index--5">
			<div class="toast__body">
				<img [src]="'assets/img/' + notification.style + '-icon.svg'" alt="toast icon" />
				{{ notification.message }}
			</div>
			<button (click)="dismissNotification()" class="button__clear margin-right__6px">
				<img src="assets/img/close icon.svg" alt="close icon" />
			</button>
		</div>
	`
})
export class ConvoyNotificationComponent implements OnInit {
	notification!: { message: string; style: string; show: boolean };
	constructor(private convoyDashboardService: ConvoyDashboardService) {}

	async ngOnInit() {
		await this.triggerNotification();
	}

	triggerNotification() {
		this.convoyDashboardService.alertStatus.subscribe(res => {
			this.notification = res;
		});
	}
	dismissNotification() {
		this.convoyDashboardService.dismissNotification();
	}
}
