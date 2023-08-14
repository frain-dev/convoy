import { Component, ElementRef, OnInit, ViewChild } from '@angular/core';
import { CommonModule } from '@angular/common';
import { GeneralService } from 'src/app/services/general/general.service';
import { ButtonComponent } from '../button/button.component';
import { DialogDirective } from '../dialog/dialog.directive';

@Component({
	selector: 'convoy-notification-modal',
	standalone: true,
	imports: [CommonModule, ButtonComponent, DialogDirective],
	templateUrl: './notification-modal.component.html',
	styleUrls: ['./notification-modal.component.scss']
})
export class NotificationModalComponent implements OnInit {
	@ViewChild('dialog', { static: true }) dialog!: ElementRef<HTMLDialogElement>;
	notification!: { message: string; show: boolean; type?: string };

	constructor(private generalService: GeneralService) {}

	async ngOnInit() {
		await this.triggerNotification();
	}

	triggerNotification() {
		this.generalService.alertStatus.subscribe(res => {
			this.notification = res;
			if (this.notification.show && this.notification.type === 'modal') this.dialog.nativeElement.showModal();
		});
	}

	dismissNotification() {
		this.generalService.dismissNotification();
		this.dialog.nativeElement.close();
	}
}
