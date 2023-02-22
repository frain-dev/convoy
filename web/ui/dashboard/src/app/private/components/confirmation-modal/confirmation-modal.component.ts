import { Component, EventEmitter, Input, OnInit, Output } from '@angular/core';
import { CommonModule } from '@angular/common';
import { ModalComponent } from 'src/app/components/modal/modal.component';
import { ButtonComponent } from 'src/app/components/button/button.component';

@Component({
	selector: 'convoy-confirmation-modal',
	standalone: true,
	imports: [CommonModule, ModalComponent, ButtonComponent],
	templateUrl: './confirmation-modal.component.html',
	styleUrls: ['./confirmation-modal.component.scss']
})
export class ConfirmationModalComponent implements OnInit {
	@Input('action') action: 'save' | 'discard' = 'discard';
	@Input('confirmText') confirmText?: string;
	@Output() closeModal = new EventEmitter<any>();
	@Output() confirmAction = new EventEmitter<any>();
	constructor() {}

	ngOnInit(): void {}
}
