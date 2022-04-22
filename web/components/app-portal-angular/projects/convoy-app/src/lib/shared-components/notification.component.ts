import { ChangeDetectionStrategy, Component, OnInit } from '@angular/core';
import { ConvoyAppService } from '../convoy-app.service';

@Component({
	selector: 'convoy-notification',
	changeDetection: ChangeDetectionStrategy.Default,
	template: `
		<div *ngIf="notification.show" class="toast toast--{{ notification.style }} z-index--5">
			<div class="toast__body">
				<svg-component [height]="32" [width]="32" [styles]="'margin-right__8px margin-top__6px'" [id]="notification.style + '-icon'"></svg-component>
				{{ notification.message }}
			</div>
			<button (click)="dismissNotification()" class="button__clear margin-right__6px margin-top__6px">
				<svg-component [height]="16" [width]="16" [id]="'close-icon'"></svg-component>
			</button>
		</div>
	`,
	styles: [
		`
			.toast {
				max-height: 46px;
                overflow-y: hidden;
			}
		`
	]
})
export class ConvoyNotificationComponent implements OnInit {
	notification!: { message: string; style: string; show: boolean };
	constructor(private convoyAppService: ConvoyAppService) {}

	async ngOnInit() {
		await this.triggerNotification();
	}

	triggerNotification() {
		this.convoyAppService.alertStatus.subscribe(res => {
			this.notification = res;
		});
	}
	dismissNotification() {
		this.convoyAppService.dismissNotification();
	}
}
