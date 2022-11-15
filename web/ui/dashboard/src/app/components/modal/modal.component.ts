import { CommonModule } from '@angular/common';
import { Component, EventEmitter, Input, OnInit, Output, ViewChild } from '@angular/core';
import { ButtonComponent } from '../button/button.component';
import { ModalDialogComponent } from '../modal-dialog/modal-dialog.component';
import { OverlayDirective } from '../overlay/overlay.directive';

@Component({
	selector: 'convoy-modal, [convoy-modal]',
	standalone: true,
	imports: [CommonModule, ButtonComponent, OverlayDirective, ModalDialogComponent],
	templateUrl: './modal.component.html',
	styleUrls: ['./modal.component.scss']
})
export class ModalComponent implements OnInit {
	@Input('position') position: 'full' | 'left' | 'right' | 'center' = 'right';
	@Input('size') size: 'sm' | 'md' | 'lg' = 'md';
	@Input('title') title!: string;
	@Input('id') id!: string;
	@Input('className') class!: string;
	@Output('closeModal') closeModal = new EventEmitter<any>();


	constructor() {}

	ngOnInit(): void {}

	// get classes(): string {
	// 	return `${this.modalType[this.position]} ${this.position === 'full' ? 'bg-[#fafafe]' : this.modalSizes[this.size]} ${this.class}`;
	// }
}


