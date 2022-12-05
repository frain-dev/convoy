import { Component, EventEmitter, OnInit, Output } from '@angular/core';
import { CommonModule } from '@angular/common';
import { ModalComponent } from 'src/app/components/modal/modal.component';
import { ButtonComponent } from 'src/app/components/button/button.component';
import { VerifyEmailService } from './verify-email.service';
import { GeneralService } from 'src/app/services/general/general.service';

@Component({
	selector: 'convoy-verify-email',
	standalone: true,
	imports: [CommonModule, ModalComponent, ButtonComponent],
	templateUrl: './verify-email.component.html',
	styleUrls: ['./verify-email.component.scss']
})
export class VerifyEmailComponent implements OnInit {
	@Output() closeModal = new EventEmitter<any>();
	isResendingEmail = false;
	constructor(private verifyEmailService: VerifyEmailService, private generalService: GeneralService) {}

	ngOnInit(): void {}

	async resendVerificationEmail() {
		this.isResendingEmail = true;
		try {
			const response = await this.verifyEmailService.resendVerificationEmail();
			this.isResendingEmail = false;
			console.log(response);
		} catch {
			this.isResendingEmail = false;
		}
	}
}
