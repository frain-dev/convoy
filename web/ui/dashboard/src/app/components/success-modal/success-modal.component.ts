import { Component, ElementRef, EventEmitter, Input, OnInit, Output, ViewChild } from '@angular/core';
import { CommonModule } from '@angular/common';
import { DialogDirective } from '../modal/modal.component';
import { ButtonComponent } from '../button/button.component';
import { GeneralService } from 'src/app/services/general/general.service';

@Component({
	selector: 'convoy-success-modal',
	standalone: true,
	imports: [CommonModule, DialogDirective, ButtonComponent],
	templateUrl: './success-modal.component.html',
	styleUrls: ['./success-modal.component.scss']
})
export class SuccessModalComponent implements OnInit {
	@ViewChild('dialog', { static: true }) dialog!: ElementRef<HTMLDialogElement>;

	notification!: { message: string; show: boolean; type?: string };
	@Output() closeModal = new EventEmitter<any>();
	@Input() successText!: string;
	@Input() buttonText!: string;
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
