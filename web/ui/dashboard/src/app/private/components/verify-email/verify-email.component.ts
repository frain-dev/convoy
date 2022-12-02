import { Component, EventEmitter, OnInit, Output } from '@angular/core';
import { CommonModule } from '@angular/common';
import { ModalComponent } from 'src/app/components/modal/modal.component';
import { ButtonComponent } from 'src/app/components/button/button.component';

@Component({
	selector: 'convoy-verify-email',
	standalone: true,
	imports: [CommonModule, ModalComponent, ButtonComponent],
	templateUrl: './verify-email.component.html',
	styleUrls: ['./verify-email.component.scss']
})
export class VerifyEmailComponent implements OnInit {
	@Output() closeModal = new EventEmitter<any>();
	constructor() {}

	ngOnInit(): void {}
}
