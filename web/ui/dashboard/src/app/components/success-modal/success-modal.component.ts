import { Component, EventEmitter, Input, OnInit, Output } from '@angular/core';
import { CommonModule } from '@angular/common';
import { ModalComponent } from '../modal/modal.component';
import { ButtonComponent } from '../button/button.component';
import { GeneralService } from 'src/app/services/general/general.service';

@Component({
	selector: 'convoy-success-modal',
	standalone: true,
	imports: [CommonModule, ModalComponent, ButtonComponent],
	templateUrl: './success-modal.component.html',
	styleUrls: ['./success-modal.component.scss']
})
export class SuccessModalComponent implements OnInit {
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
		});
	}
	dismissNotification() {
		this.generalService.dismissNotification();
	}
}
