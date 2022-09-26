import { Component, EventEmitter, Input, OnInit, Output } from '@angular/core';
import { CommonModule } from '@angular/common';
import { ModalComponent } from '../modal/modal.component';
import { ButtonComponent } from '../button/button.component';

@Component({
	selector: 'convoy-success-modal',
	standalone: true,
	imports: [CommonModule, ModalComponent, ButtonComponent],
	templateUrl: './success-modal.component.html',
	styleUrls: ['./success-modal.component.scss']
})
export class SuccessModalComponent implements OnInit {
	@Output() closeModal = new EventEmitter<any>();
    @Input() successText!: string;
    @Input() buttonText!: string;
	constructor() {}

	ngOnInit(): void {}
}
